package vm

import stepvm "github.com/wyvernzora/k2/tools/internal/step/vm"

func (c *vmStopCmd) Run(ctx *Runtime) error {
	return vmRunner(ctx).Stop(stepvm.StopOptions{ID: c.ID, All: c.All})
}
