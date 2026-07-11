package vm

func (c *vmConsoleCmd) Run(ctx *Runtime) error { return vmRunner(ctx).Console(c.ID) }
