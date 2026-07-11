package vm

func (c *vmMonitorCmd) Run(ctx *Runtime) error { return vmRunner(ctx).Monitor(c.ID) }
