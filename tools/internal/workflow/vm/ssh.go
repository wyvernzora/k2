package vm

func (c *vmSSHCmd) Run(ctx *Runtime) error { return vmRunner(ctx).SSH(c.ID) }
