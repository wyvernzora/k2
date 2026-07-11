package vm

import stepvm "github.com/wyvernzora/k2/tools/internal/step/vm"

func (c *vmDeleteCmd) Run(ctx *Runtime) error {
	return vmRunner(ctx).Delete(stepvm.DeleteOptions{ID: c.ID, Force: c.Force, All: c.All})
}
