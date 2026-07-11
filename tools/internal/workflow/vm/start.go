package vm

import stepvm "github.com/wyvernzora/k2/tools/internal/step/vm"

func (c *vmStartCmd) Run(ctx *Runtime) error {
	return vmRunner(ctx).Start(stepvm.StartOptions{ID: c.ID, Sudo: c.Sudo})
}
