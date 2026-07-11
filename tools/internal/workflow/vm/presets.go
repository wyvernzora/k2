package vm

func (c *vmPresetsCmd) Run(ctx *Runtime) error { return vmRunner(ctx).Presets() }
