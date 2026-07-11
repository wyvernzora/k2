package vm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func listIP(meta Metadata) string {
	if !isRunning(meta) {
		return "-"
	}
	ip, err := bestGuestIPv4(meta, 1)
	if err != nil {
		return "-"
	}
	return ip.Address
}

func (r Runner) Create(opts CreateOptions) error {
	meta, preset, rawXZ, err := r.prepareCreate(opts)
	if err != nil {
		return err
	}
	if err := r.createDisks(meta, preset, rawXZ); err != nil {
		return err
	}
	if err := writeMetadata(meta); err != nil {
		return err
	}
	r.printCreateSummary(meta)
	if opts.Start {
		return r.start(meta, opts.Sudo)
	}
	return nil
}

func (r Runner) prepareCreate(opts CreateOptions) (Metadata, Preset, string, error) {
	if err := checkCreateCommands(); err != nil {
		return Metadata{}, Preset{}, "", err
	}
	id, err := resolveVMID(opts.ID)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	vmDir := dir(r.RepoRoot, id)
	if _, err := os.Stat(vmDir); err == nil {
		// A dir without vm.json is debris from a create interrupted during
		// disk imaging (metadata is written last). No command can list or
		// delete it, so reclaim it here instead of erroring forever.
		if _, metaErr := os.Stat(filepath.Join(vmDir, "vm.json")); errors.Is(metaErr, os.ErrNotExist) {
			r.logf("removing partial VM directory from an interrupted create: %s", vmDir)
			if err := os.RemoveAll(vmDir); err != nil {
				return Metadata{}, Preset{}, "", err
			}
		} else {
			return Metadata{}, Preset{}, "", fmt.Errorf("VM directory already exists: %s", vmDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Metadata{}, Preset{}, "", err
	}

	preset, target, err := resolvePreset(r.RepoRoot, opts.Preset)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	if opts.MemoryMB != 0 {
		preset.MemoryMB = opts.MemoryMB
	}
	rawXZ, err := resolveArtifact(r.RepoRoot, opts.RawXZ, target)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	ports, err := resolveCreatePorts(r.RepoRoot, preset, opts)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	extraDisks, err := resolveExtraDisks(preset, opts)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}

	if err := os.MkdirAll(vmDir, 0o755); err != nil {
		return Metadata{}, Preset{}, "", err
	}
	kairosQCOW2 := filepath.Join(vmDir, "kairos-"+id+".qcow2")
	persistentQCOW2 := filepath.Join(vmDir, "persistent-"+id+".qcow2")

	meta := Metadata{
		Backend:         "qemu",
		ID:              id,
		Name:            "k2-qemu-" + id,
		Preset:          opts.Preset,
		Target:          target,
		RawXZ:           rawXZ,
		VMDir:           vmDir,
		KairosQCOW2:     kairosQCOW2,
		PersistentQCOW2: persistentQCOW2,
		SSHPort:         ports.ssh,
		APIPort:         ports.api,
		MonitorPort:     ports.monitor,
		QGAPort:         ports.qga,
		NetworkMode:     preset.Network.Mode,
		MACAddress:      deterministicMACAddress(id),
		MemoryMB:        preset.MemoryMB,
		CPUs:            preset.CPUs,
		PIDFile:         filepath.Join(vmDir, "qemu.pid"),
		QEMULog:         filepath.Join(vmDir, "qemu.log"),
		ConsoleLog:      filepath.Join(vmDir, "console.log"),
		ConsoleSocket:   filepath.Join(vmDir, "console.sock"),
	}
	meta.ExtraDisks = extraDisksForVM(id, vmDir, extraDisks)
	return meta, preset, rawXZ, nil
}

type createPorts struct {
	ssh, api, monitor, qga int
}

// resolveCreatePorts claims the four per-VM ports. Ports already recorded in
// ANY VM's metadata are off-limits even when currently unbound: a stopped VM
// must keep exclusive claim to its ports or a later create silently aliases
// the two VMs' monitor/QGA identity.
func resolveCreatePorts(repoRoot string, preset Preset, opts CreateOptions) (createPorts, error) {
	taken, err := recordedPorts(repoRoot)
	if err != nil {
		return createPorts{}, err
	}
	ssh, err := resolveOptionalPort(preset.Network.Mode, opts.SSHPort, forwardPortSpec(preset, "ssh", ""), taken)
	if err != nil {
		return createPorts{}, err
	}
	taken[ssh] = true
	api, err := resolveOptionalPort(preset.Network.Mode, opts.APIPort, forwardPortSpec(preset, "k8s-api", ""), taken)
	if err != nil {
		return createPorts{}, err
	}
	taken[api] = true
	monitor, err := findFreePort(24000, 24999, taken)
	if err != nil {
		return createPorts{}, err
	}
	taken[monitor] = true
	qga, err := findFreePort(25000, 25999, taken)
	if err != nil {
		return createPorts{}, err
	}
	return createPorts{ssh: ssh, api: api, monitor: monitor, qga: qga}, nil
}

func (r Runner) createDisks(meta Metadata, preset Preset, rawXZ string) error {
	r.logf("using preset %s: %s", meta.Preset, preset.Description)
	r.logf("using artifact %s", rawXZ)
	if err := convertRawXZ(rawXZ, filepath.Join(meta.VMDir, "kairos-"+meta.ID+".raw"), meta.KairosQCOW2); err != nil {
		return err
	}
	if !preset.PersistentDisk.Enabled {
		return r.createExtraDisks(meta)
	}
	r.logf("creating persistent disk %s (%dM)", meta.PersistentQCOW2, preset.PersistentDisk.SizeMB)
	if err := runCommand(exec.Command("qemu-img", "create", "-q", "-f", "qcow2", meta.PersistentQCOW2, fmt.Sprintf("%dM", preset.PersistentDisk.SizeMB))); err != nil {
		return err
	}
	return r.createExtraDisks(meta)
}

func (r Runner) createExtraDisks(meta Metadata) error {
	for _, disk := range meta.ExtraDisks {
		r.logf("creating extra disk %s (%dM)", disk.QCOW2, disk.SizeMB)
		if err := runCommand(exec.Command("qemu-img", "create", "-q", "-f", "qcow2", disk.QCOW2, fmt.Sprintf("%dM", disk.SizeMB))); err != nil {
			return err
		}
	}
	return nil
}

func resolveExtraDisks(preset Preset, opts CreateOptions) (ExtraDisks, error) {
	extra := preset.ExtraDisks
	if opts.ExtraDisks != 0 {
		extra.Count = opts.ExtraDisks
		if opts.ExtraDiskSizeMB != 0 {
			extra.SizeMB = opts.ExtraDiskSizeMB
		}
	} else if opts.ExtraDiskSizeMB != 0 {
		if extra.Count == 0 {
			return ExtraDisks{}, fmt.Errorf("extra disk count must be > 0 when extra disk size is set")
		}
		extra.SizeMB = opts.ExtraDiskSizeMB
	}
	if extra.Count < 0 {
		return ExtraDisks{}, fmt.Errorf("extra disk count must be >= 0")
	}
	if extra.SizeMB < 0 {
		return ExtraDisks{}, fmt.Errorf("extra disk size must be >= 0")
	}
	if extra.Count == 0 {
		return ExtraDisks{}, nil
	}
	if extra.SizeMB == 0 {
		return ExtraDisks{}, fmt.Errorf("extra disk size must be > 0 when extra disks are requested")
	}
	return extra, nil
}

func extraDisksForVM(id string, vmDir string, extra ExtraDisks) []Disk {
	disks := make([]Disk, 0, extra.Count)
	for i := range extra.Count {
		n := i + 1
		disks = append(disks, Disk{
			ID:     fmt.Sprintf("extra-%s-%d", id, n),
			QCOW2:  filepath.Join(vmDir, fmt.Sprintf("extra-%s-%d.qcow2", id, n)),
			SizeMB: extra.SizeMB,
		})
	}
	return disks
}

func (r Runner) printCreateSummary(meta Metadata) {
	r.successf("created %s", meta.Name)
	r.logf("directory: %s", meta.VMDir)
	r.logf("ssh: k2-tools vm ssh %s", meta.ID)
	if meta.APIPort != 0 {
		r.logf("api: https://127.0.0.1:%d", meta.APIPort)
	}
	r.logf("console: k2-tools vm console %s", meta.ID)
}

func (r Runner) resolveTargetVMs(id string, all bool) ([]Metadata, error) {
	switch {
	case all && id != "":
		return nil, fmt.Errorf("pass either a VM id or --all, not both")
	case all:
		return listMetadata(r.RepoRoot)
	case id == "":
		return nil, fmt.Errorf("missing VM id; pass an id or --all")
	default:
		meta, err := loadMetadata(r.RepoRoot, id)
		if err != nil {
			return nil, err
		}
		return []Metadata{meta}, nil
	}
}

func (r Runner) confirmDelete(metas []Metadata) error {
	if len(metas) == 1 {
		fmt.Fprintf(r.stderr(), "Delete VM %s and %s? [y/N] ", metas[0].Name, metas[0].VMDir)
	} else {
		fmt.Fprintf(r.stderr(), "Delete %d VMs under %s? [y/N] ", len(metas), root(r.RepoRoot))
	}

	var answer string
	if _, err := fmt.Fscanln(r.stdin(), &answer); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if answer != "y" && answer != "Y" {
		return fmt.Errorf("delete cancelled")
	}
	return nil
}

func (r Runner) runInteractive(cmd *exec.Cmd) error {
	cmd.Stdin = r.stdin()
	cmd.Stdout = r.stdout()
	cmd.Stderr = r.stderr()
	return cmd.Run()
}
