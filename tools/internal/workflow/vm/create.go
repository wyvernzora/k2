package vm

import stepvm "github.com/wyvernzora/k2/tools/internal/step/vm"

func (c *vmCreateCmd) Run(ctx *Runtime) error {
	return vmRunner(ctx).Create(stepvm.CreateOptions{
		Preset: c.Preset, ID: c.ID, RawXZ: c.RawXZ, SSHPort: c.SSHPort, APIPort: c.APIPort,
		ExtraDisks: c.ExtraDisks, ExtraDiskSizeMB: c.ExtraDiskSizeMB, Start: c.Start, Sudo: c.Sudo,
	})
}
