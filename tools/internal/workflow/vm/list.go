package vm

func (c *vmListCmd) Run(ctx *Runtime) error { return vmRunner(ctx).List() }
