package flash

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

// Runner orchestrates the rpi4cb flash flow end-to-end. It mirrors the
// shape of internal/vm.Runner: assembled by the CLI layer with the
// shared repo root + reporter, plus stdin/stdout/stderr for prompts.
type Runner struct {
	RepoRoot string
	Reporter *ui.Reporter
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

// Rpi4cbOptions controls one invocation of the rpi4cb flash flow.
type Rpi4cbOptions struct {
	EMMCDiskID     string
	NVMeDiskID     string
	ZeroNVMe       bool
	SkipVerify     bool
	SkipRpiboot    bool
	Yes            bool
	RpibootTimeout time.Duration
}

func (o *Rpi4cbOptions) applyDefaults() {
	if o.RpibootTimeout == 0 {
		o.RpibootTimeout = defaultRpibootTimeout
	}
}

const (
	defaultRpibootTimeout        = 60 * time.Second
	nvmeZeroMiB           uint64 = 64
)

// rpi4cbState carries the imperative state shared across workflow
// step closures: target manifest, platform handle, and resolved disks.
// Per-phase helpers read and mutate this through a pointer so they
// avoid globals while keeping the orchestrator flat.
type rpi4cbState struct {
	target  ResolvedTarget
	plat    Platform
	before  []Disk
	emmc    Disk
	nvme    Disk
	hasNVMe bool
}

// Rpi4cb runs the full flow as a declarative ui.Workflow. The
// orchestrator is a flat sequence of per-phase helper calls; each
// helper owns its conditional-skip logic and contributes its slice
// of workflow steps.
func (r Runner) Rpi4cb(parentCtx context.Context, opts Rpi4cbOptions) error {
	opts.applyDefaults()

	// Plumb a cancellable context whose cancel func the Step layer
	// invokes on Ctrl-C. Bubbletea puts the terminal in raw mode and
	// would otherwise swallow Ctrl-C silently, leaving any active
	// `sudo dd` running. This makes the in-bubbletea key event reach
	// every CommandContext-spawned subprocess in this flow.
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	r.Reporter.SetInterruptCancel(cancel)
	defer r.Reporter.SetInterruptCancel(nil)

	st := &rpi4cbState{}
	wf := ui.NewWorkflow(r.Reporter)

	r.addRpi4cbPrelude(wf, st)
	r.addRpi4cbDiskResolution(wf, st, opts)
	r.addRpi4cbPlanConfirm(wf, st, opts)
	r.addRpi4cbWrite(wf, st)
	r.addRpi4cbVerify(wf, st, opts)
	r.addRpi4cbZeroNVMe(wf, st, opts)
	r.addRpi4cbCleanup(wf, st)
	r.addRpi4cbFinalBanner(wf, st, opts)

	return wf.Execute(ctx)
}

// addRpi4cbPrelude resolves the target manifest, prints the
// "Flashing rpi4cb" header + target KVs, resolves the host platform,
// and acquires the sudo handle the dd-based phases need.
func (r Runner) addRpi4cbPrelude(wf *ui.Workflow, st *rpi4cbState) {
	wf.Task("Resolve target", func(ctx context.Context) error {
		var err error
		st.target, err = ResolveRpi4cbTarget(r.RepoRoot)
		return err
	})

	wf.Section("Flashing rpi4cb")

	wf.KeyValuesFn(func() []ui.KV {
		return []ui.KV{
			{Key: "Target", Value: st.target.Manifest.Target},
			{Key: "Image", Value: st.target.Manifest.Compressed.File},
			{Key: "Size", Value: fmt.Sprintf("%s raw / %s compressed",
				ui.HumanBytes(st.target.Manifest.Raw.SizeBytes),
				ui.HumanBytes(st.target.Manifest.Compressed.SizeBytes))},
		}
	})

	wf.Task("Resolve platform", func(ctx context.Context) error {
		var err error
		st.plat, err = CurrentPlatform()
		return err
	})

	wf.Sudo("dd needs root for raw disk access")
}

// addRpi4cbDiskResolution covers the two disjoint disk-resolution
// paths: explicit operator-supplied IDs vs. auto-detect via rpiboot
// snapshot diff. Exactly one set of steps actually runs at execute
// time based on the Unless() gates.
func (r Runner) addRpi4cbDiskResolution(wf *ui.Workflow, st *rpi4cbState, opts Rpi4cbOptions) {
	explicit := opts.EMMCDiskID != ""
	autoDetect := !explicit
	runRpiboot := autoDetect && !opts.SkipRpiboot

	wf.Task("Resolve explicit disks", func(ctx context.Context) error {
		return resolveExplicitDisks(ctx, st, opts)
	}).Unless(autoDetect)

	wf.Section("rpiboot").Unless(explicit)

	wf.Task("Snapshot disks", func(ctx context.Context) error {
		var err error
		st.before, err = st.plat.EnumerateExternalDisks(ctx)
		if err != nil {
			return fmt.Errorf("flash: snapshot disks: %w", err)
		}
		return nil
	}).Unless(explicit)

	wf.Note(ui.NoteInfo,
		"Hold the nRPIBOOT button on the ComputeBlade carrier and connect USB-C now.").
		Unless(!runRpiboot)

	wf.Shell("Running rpiboot", func(ctx context.Context, sh ui.Step) error {
		return RunRpiboot(ctx, sh)
	}).Unless(!runRpiboot)

	wf.Shell("Wait for new disks to enumerate", func(ctx context.Context, sh ui.Step) error {
		return waitForNewDisksStep(ctx, sh, st, opts.RpibootTimeout)
	}).Unless(explicit)

	wf.KeyValuesFn(func() []ui.KV {
		kvs := []ui.KV{{Key: "Detected eMMC", Value: fmt.Sprintf("%s — %s", st.emmc.Path, st.emmc.Description)}}
		if st.hasNVMe {
			kvs = append(kvs, ui.KV{Key: "Detected NVMe", Value: fmt.Sprintf("%s — %s", st.nvme.Path, st.nvme.Description)})
		}
		return kvs
	}).Unless(explicit)
}

// resolveExplicitDisks looks up the operator-supplied --emmc / --nvme
// IDs on the host platform. Factored out so the orchestrator stays
// flat and cyclop-friendly.
func resolveExplicitDisks(ctx context.Context, st *rpi4cbState, opts Rpi4cbOptions) error {
	var err error
	st.emmc, err = st.plat.DiskByID(ctx, opts.EMMCDiskID)
	if err != nil {
		return err
	}
	if opts.NVMeDiskID != "" {
		st.nvme, err = st.plat.DiskByID(ctx, opts.NVMeDiskID)
		if err != nil {
			return err
		}
		st.hasNVMe = true
	}
	return nil
}

// waitForNewDisksStep blocks until rpiboot enumerates the new eMMC
// (and optional NVMe), classifies them, and stores the result in the
// shared state struct.
func waitForNewDisksStep(ctx context.Context, sh ui.Step, st *rpi4cbState, timeout time.Duration) error {
	newDisks, err := WaitForNewDisks(ctx, st.plat, st.before, timeout)
	if err != nil {
		return err
	}
	class, err := Classify(newDisks)
	if err != nil {
		return fmt.Errorf("%w (use --emmc / --nvme to disambiguate)", err)
	}
	if !class.HasEMMC {
		return fmt.Errorf("flash: no eMMC-sized disk in the new set (%s); supply --emmc explicitly", describeDisks(newDisks))
	}
	st.emmc, st.nvme, st.hasNVMe = class.EMMC, class.NVMe, class.HasNVMe
	sh.Successf("detected %d new disk(s)", len(newDisks))
	return nil
}

// addRpi4cbPlanConfirm prints the plan summary the operator sees right
// before destruction and (unless --yes) the FLASH confirmation prompt.
func (r Runner) addRpi4cbPlanConfirm(wf *ui.Workflow, st *rpi4cbState, opts Rpi4cbOptions) {
	wf.Section("Plan")

	wf.KeyValuesFn(func() []ui.KV {
		return planFields(st.target, st.emmc, st.nvme, st.hasNVMe, opts)
	})

	wf.Confirm("Type FLASH to confirm:", "FLASH").Unless(opts.Yes)
}

// addRpi4cbWrite unmounts the eMMC, streams the decompressed image to
// it via sudo dd, and issues a final sync so the kernel flushes its
// write buffers before any verify pass reads back.
func (r Runner) addRpi4cbWrite(wf *ui.Workflow, st *rpi4cbState) {
	wf.Section("Write")

	wf.Shell("Unmount eMMC", func(ctx context.Context, sh ui.Step) error {
		if err := st.plat.Unmount(ctx, st.emmc); err != nil {
			return err
		}
		sh.Successf("unmounted %s", st.emmc.Path)
		return nil
	})

	wf.Progress("Write image",
		func() uint64 { return st.target.Manifest.Raw.SizeBytes },
		func(ctx context.Context, p ui.Progress) error {
			if err := WriteRawDisk(ctx, p, st.target.CompressedPath, st.target.Manifest.Raw.SizeBytes, st.emmc.RawPath); err != nil {
				return err
			}
			p.Successf("wrote %s", ui.HumanBytes(st.target.Manifest.Raw.SizeBytes))
			return nil
		})

	wf.Shell("Flush write buffers (sync)", func(ctx context.Context, sh ui.Step) error {
		return SyncDisks(ctx, sh)
	})
}

// addRpi4cbVerify re-reads the newly-written eMMC and confirms its
// hash matches the manifest. Skipped under --skip-verify.
func (r Runner) addRpi4cbVerify(wf *ui.Workflow, st *rpi4cbState, opts Rpi4cbOptions) {
	wf.Section("Verify").Unless(opts.SkipVerify)

	wf.Progress("Verify hash",
		func() uint64 { return st.target.Manifest.Raw.SizeBytes },
		func(ctx context.Context, p ui.Progress) error {
			if err := VerifyRawDisk(ctx, p, st.emmc.RawPath, st.target.Manifest.Raw.SizeBytes, st.target.Manifest.Raw.SHA256); err != nil {
				return err
			}
			p.Successf("hash matches manifest")
			return nil
		}).Unless(opts.SkipVerify)
}

// addRpi4cbZeroNVMe optionally zeroes the head of the NVMe so the
// node's first boot can't reuse a stale filesystem. Gated by both
// the --zero-nvme flag and the presence of a detected NVMe.
func (r Runner) addRpi4cbZeroNVMe(wf *ui.Workflow, st *rpi4cbState, opts Rpi4cbOptions) {
	wf.Section("Zero NVMe").Unless(!opts.ZeroNVMe)

	wf.Shell("Zero head of NVMe", func(ctx context.Context, sh ui.Step) error {
		return zeroNVMeStep(ctx, sh, st)
	}).Unless(!opts.ZeroNVMe)
}

// zeroNVMeStep handles the "--zero-nvme set but no NVMe detected"
// degenerate case quietly (warn-and-skip) and zeroes the head of the
// NVMe device otherwise.
func zeroNVMeStep(ctx context.Context, sh ui.Step, st *rpi4cbState) error {
	if !st.hasNVMe {
		sh.Warnf("--zero-nvme set but no NVMe detected; skipping")
		return nil
	}
	if err := st.plat.Unmount(ctx, st.nvme); err != nil {
		sh.Warnf("could not unmount NVMe: %v (skipping zero)", err)
		return nil
	}
	if err := ZeroDiskHead(ctx, sh, st.nvme.RawPath, nvmeZeroMiB); err != nil {
		return err
	}
	sh.Successf("zeroed first %d MiB of %s", nvmeZeroMiB, st.nvme.Path)
	return nil
}

// addRpi4cbCleanup ejects the eMMC (and NVMe if present) so the
// operator can unplug the carrier without macOS popping the "disk
// not ejected properly" toast.
func (r Runner) addRpi4cbCleanup(wf *ui.Workflow, st *rpi4cbState) {
	wf.Section("Cleanup")

	wf.Shell("Eject disks", func(ctx context.Context, sh ui.Step) error {
		return ejectDisksStep(ctx, sh, st)
	})
}

// ejectDisksStep ejects eMMC + NVMe. Eject failures degrade to a
// warning rather than an error — by this point the write is done
// and the operator can manually unmount.
func ejectDisksStep(ctx context.Context, sh ui.Step, st *rpi4cbState) error {
	if err := st.plat.Eject(ctx, st.emmc); err != nil {
		sh.Warnf("could not eject %s: %v", st.emmc.Path, err)
		return nil
	}
	if st.hasNVMe {
		if err := st.plat.Eject(ctx, st.nvme); err != nil {
			sh.Warnf("could not eject %s: %v", st.nvme.Path, err)
			return nil
		}
		sh.Successf("ejected %s + %s", st.emmc.Path, st.nvme.Path)
		return nil
	}
	sh.Successf("ejected %s", st.emmc.Path)
	return nil
}

// addRpi4cbFinalBanner prints the "Flash complete" success banner.
func (r Runner) addRpi4cbFinalBanner(wf *ui.Workflow, st *rpi4cbState, opts Rpi4cbOptions) {
	wf.BannerFn(ui.BannerSuccess, func() []string {
		return []string{
			"Flash complete",
			fmt.Sprintf("%s written%s", ui.HumanBytes(st.target.Manifest.Raw.SizeBytes), verifySuffix(opts.SkipVerify)),
			"Safe to unplug or reboot the ComputeBlade.",
		}
	})
}

func verifySuffix(skip bool) string {
	if skip {
		return " (verify skipped)"
	}
	return " + verified"
}

// planFields renders the multi-line plan summary the operator sees
// right before the FLASH prompt. Kept boring and factual — surprises
// here are failures of the auto-detect, not stylistic choices.
func planFields(target ResolvedTarget, emmc Disk, nvme Disk, hasNVMe bool, opts Rpi4cbOptions) []ui.KV {
	pairs := []ui.KV{
		{Key: "Image", Value: fmt.Sprintf("%s (%s)", target.Manifest.Compressed.File, ui.HumanBytes(target.Manifest.Raw.SizeBytes))},
		{Key: "eMMC", Value: fmt.Sprintf("%s  (%s) — WILL BE OVERWRITTEN", emmc.Path, emmc.Description)},
	}
	if opts.ZeroNVMe && hasNVMe {
		pairs = append(pairs, ui.KV{Key: "NVMe", Value: fmt.Sprintf("%s  (%s) — first %d MiB will be ZEROED", nvme.Path, nvme.Description, nvmeZeroMiB)})
	} else if hasNVMe {
		pairs = append(pairs, ui.KV{Key: "NVMe", Value: fmt.Sprintf("%s  (%s) — not touched (--zero-nvme not set)", nvme.Path, nvme.Description)})
	}
	if opts.SkipVerify {
		pairs = append(pairs, ui.KV{Key: "Verify", Value: "SKIPPED (--skip-verify)"})
	}
	return pairs
}

func describeDisks(ds []Disk) string {
	if len(ds) == 0 {
		return "(no disks)"
	}
	parts := make([]string, 0, len(ds))
	for _, d := range ds {
		parts = append(parts, fmt.Sprintf("%s (%s)", d.Path, ui.HumanBytes(d.SizeBytes)))
	}
	return strings.Join(parts, ", ")
}
