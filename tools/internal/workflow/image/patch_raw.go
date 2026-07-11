package image

import (
	"os"

	stepimage "github.com/wyvernzora/k2/tools/internal/step/image"
)

func (c *imagePatchRawCmd) Run(ctx *Runtime, parent *imageCmd) error {
	runCtx, done := buildCommandContext()
	defer done()
	planner, err := loadImagePlanner(parent.imageGlobals())
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}
	return stepimage.RawPatcher{Context: runCtx, Stdout: os.Stdout, Stderr: os.Stderr}.Patch(c.Raw, resolved)
}
