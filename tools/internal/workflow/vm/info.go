package vm

func (c *vmInfoCmd) Run(ctx *Runtime) error { return vmRunner(ctx).Info(c.ID) }
