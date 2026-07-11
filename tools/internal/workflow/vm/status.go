package vm

func (c *vmStatusCmd) Run(ctx *Runtime) error { return vmRunner(ctx).Status(c.ID) }
