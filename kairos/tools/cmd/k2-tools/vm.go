package main

import "github.com/wyvernzora/k2/kairos/tools/internal/vm"

type vmCmd struct {
	Create  vmCreateCmd  `cmd:"" help:"Create a local QEMU test VM."`
	Presets vmPresetsCmd `cmd:"" help:"List available VM presets."`
	List    vmListCmd    `cmd:"" help:"List local QEMU test VMs."`
	Info    vmInfoCmd    `cmd:"" help:"Show local QEMU test VM metadata."`
	Start   vmStartCmd   `cmd:"" help:"Start a local QEMU test VM."`
	Stop    vmStopCmd    `cmd:"" help:"Stop a local QEMU test VM."`
	Status  vmStatusCmd  `cmd:"" help:"Show local QEMU test VM status."`
	SSH     vmSSHCmd     `cmd:"ssh" help:"SSH into a local QEMU test VM."`
	Console vmConsoleCmd `cmd:"" help:"Follow local QEMU test VM console output."`
	Monitor vmMonitorCmd `cmd:"" help:"Connect to the local QEMU monitor."`
	Delete  vmDeleteCmd  `cmd:"" help:"Delete a local QEMU test VM."`
}

type vmCreateCmd struct {
	Preset  string `arg:"" optional:"" default:"qemu-user" help:"Preset name."`
	ID      string `name:"id" help:"VM id. Defaults to a random 8-character id."`
	RawXZ   string `name:"raw-xz" help:"Raw .raw.xz artifact to convert. Overrides local/cache/S3 lookup." type:"path"`
	SSHPort string `name:"ssh-port" default:"" help:"Override preset SSH host forward."`
	APIPort string `name:"api-port" default:"" help:"Override preset Kubernetes API host forward."`
	Start   bool   `name:"start" help:"Start the VM after creation."`
	Sudo    bool   `name:"sudo" env:"K2_TOOLS_VM_SUDO_QEMU" help:"Launch QEMU through sudo. Useful for macOS vmnet-shared networking."`
}

type vmPresetsCmd struct{}
type vmListCmd struct{}
type vmInfoCmd struct {
	ID string `arg:"" help:"VM id or vm-<id> directory."`
}
type vmStartCmd struct {
	ID   string `arg:"" help:"VM id or vm-<id> directory."`
	Sudo bool   `name:"sudo" env:"K2_TOOLS_VM_SUDO_QEMU" help:"Launch QEMU through sudo. Useful for macOS vmnet-shared networking."`
}
type vmStopCmd struct {
	All bool   `name:"all" help:"Stop all local QEMU test VMs."`
	ID  string `arg:"" optional:"" help:"VM id or vm-<id> directory."`
}
type vmStatusCmd struct {
	ID string `arg:"" help:"VM id or vm-<id> directory."`
}
type vmSSHCmd struct {
	ID string `arg:"" help:"VM id or vm-<id> directory."`
}
type vmConsoleCmd struct {
	ID string `arg:"" help:"VM id or vm-<id> directory."`
}
type vmMonitorCmd struct {
	ID string `arg:"" help:"VM id or vm-<id> directory."`
}
type vmDeleteCmd struct {
	Force bool   `name:"force" help:"Delete without prompting."`
	All   bool   `name:"all" help:"Delete all local QEMU test VMs."`
	ID    string `arg:"" optional:"" help:"VM id or vm-<id> directory."`
}

func (c *vmPresetsCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Presets()
}

func (c *vmListCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).List()
}

func (c *vmCreateCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Create(vm.CreateOptions{
		Preset:  c.Preset,
		ID:      c.ID,
		RawXZ:   c.RawXZ,
		SSHPort: c.SSHPort,
		APIPort: c.APIPort,
		Start:   c.Start,
		Sudo:    c.Sudo,
	})
}

func (c *vmInfoCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Info(c.ID)
}

func (c *vmStartCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Start(vm.StartOptions{ID: c.ID, Sudo: c.Sudo})
}

func (c *vmStopCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Stop(vm.StopOptions{ID: c.ID, All: c.All})
}

func (c *vmStatusCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Status(c.ID)
}

func (c *vmSSHCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).SSH(c.ID)
}

func (c *vmConsoleCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Console(c.ID)
}

func (c *vmMonitorCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Monitor(c.ID)
}

func (c *vmDeleteCmd) Run(ctx *runContext) error {
	return vmRunner(ctx).Delete(vm.DeleteOptions{ID: c.ID, Force: c.Force, All: c.All})
}

func vmRunner(ctx *runContext) vm.Runner {
	return vm.Runner{RepoRoot: ctx.repoRoot, Reporter: reporter}
}
