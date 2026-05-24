package vm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func (r Runner) Presets() error {
	presets, err := listPresets(r.RepoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(presets))
	for _, preset := range presets {
		rows = append(rows, []string{preset.name, preset.Description})
	}
	r.Reporter.Table([]string{"PRESET", "DESCRIPTION"}, rows)
	return nil
}

func (r Runner) List() error {
	metas, err := listMetadata(r.RepoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(metas))
	for _, meta := range metas {
		state := "stopped"
		if isRunning(meta) {
			state = "running"
		}
		rows = append(rows, []string{meta.ID, meta.Preset, state, listIP(meta), meta.VMDir})
	}
	r.Reporter.Table([]string{"ID", "PRESET", "STATE", "IP", "DIR"}, rows)
	return nil
}

func listIP(meta Metadata) string {
	if !isRunning(meta) {
		return "-"
	}
	ip, err := firstGuestIPv4(meta)
	if err != nil {
		return "-"
	}
	return ip
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
		return Metadata{}, Preset{}, "", fmt.Errorf("VM directory already exists: %s", vmDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Metadata{}, Preset{}, "", err
	}

	preset, target, err := resolvePreset(r.RepoRoot, opts.Preset)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	rawXZ, err := resolveArtifact(r.RepoRoot, opts.RawXZ, target)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	sshPort, err := resolveOptionalPort(preset.Network.Mode, opts.SSHPort, forwardPortSpec(preset, "ssh", ""))
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	apiPort, err := resolveOptionalPort(preset.Network.Mode, opts.APIPort, forwardPortSpec(preset, "k8s-api", ""))
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	monitorPort, err := findFreePort(24000, 24999)
	if err != nil {
		return Metadata{}, Preset{}, "", err
	}
	qgaPort, err := findFreePort(25000, 25999)
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
		SSHPort:         sshPort,
		APIPort:         apiPort,
		MonitorPort:     monitorPort,
		QGAPort:         qgaPort,
		NetworkMode:     preset.Network.Mode,
		MACAddress:      deterministicMACAddress(id),
		MemoryMB:        preset.MemoryMB,
		CPUs:            preset.CPUs,
		PIDFile:         filepath.Join(vmDir, "qemu.pid"),
		QEMULog:         filepath.Join(vmDir, "qemu.log"),
		ConsoleLog:      filepath.Join(vmDir, "console.log"),
		ConsoleSocket:   filepath.Join(vmDir, "console.sock"),
	}
	return meta, preset, rawXZ, nil
}

func (r Runner) createDisks(meta Metadata, preset Preset, rawXZ string) error {
	r.logf("using preset %s: %s", meta.Preset, preset.Description)
	r.logf("using artifact %s", rawXZ)
	if err := convertRawXZ(rawXZ, filepath.Join(meta.VMDir, "kairos-"+meta.ID+".raw"), meta.KairosQCOW2); err != nil {
		return err
	}
	if !preset.PersistentDisk.Enabled {
		return nil
	}
	r.logf("creating persistent disk %s (%dM)", meta.PersistentQCOW2, preset.PersistentDisk.SizeMB)
	return runCommand(exec.Command("qemu-img", "create", "-f", "qcow2", meta.PersistentQCOW2, fmt.Sprintf("%dM", preset.PersistentDisk.SizeMB)))
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

func (r Runner) Info(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	r.Reporter.KeyValues(infoFields(meta)...)
	return nil
}

func (r Runner) Start(opts StartOptions) error {
	meta, err := loadMetadata(r.RepoRoot, opts.ID)
	if err != nil {
		return err
	}
	return r.start(meta, opts.Sudo)
}

func (r Runner) Stop(opts StopOptions) error {
	metas, err := r.resolveTargetVMs(opts.ID, opts.All)
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		r.logf("no VMs found")
		return nil
	}
	for _, meta := range metas {
		if err := r.stop(meta); err != nil {
			return fmt.Errorf("stop %s: %w", meta.ID, err)
		}
	}
	return nil
}

func (r Runner) Status(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	r.Reporter.KeyValues(statusFields(meta)...)
	return nil
}

func (r Runner) SSH(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	if meta.SSHPort != 0 {
		cmd.Args = append(cmd.Args, "-p", strconv.Itoa(meta.SSHPort), "kairos@127.0.0.1")
	} else {
		ip, err := firstGuestIPv4(meta)
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, "kairos@"+ip)
	}
	return r.runInteractive(cmd)
}

func (r Runner) Console(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	return r.attachConsole(meta)
}

func (r Runner) Monitor(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	return r.runInteractive(exec.Command("nc", "127.0.0.1", strconv.Itoa(meta.MonitorPort)))
}

func (r Runner) Delete(opts DeleteOptions) error {
	metas, err := r.resolveTargetVMs(opts.ID, opts.All)
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		r.logf("no VMs found")
		return nil
	}
	if !opts.Force {
		if err := r.confirmDelete(metas); err != nil {
			return err
		}
	}

	for _, meta := range metas {
		if err := r.stop(meta); err != nil {
			return fmt.Errorf("stop %s before delete: %w", meta.ID, err)
		}
		if err := os.RemoveAll(meta.VMDir); err != nil {
			return fmt.Errorf("delete %s: %w", meta.ID, err)
		}
		r.successf("deleted %s", meta.ID)
	}
	return nil
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
