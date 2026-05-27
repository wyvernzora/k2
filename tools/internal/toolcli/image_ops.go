package toolcli

import (
	"os"

	artifactinspect "github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/inspect"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/rawpatch"
)

func (c *imagePatchRawCmd) Run(ctx *runContext, parent *imageCmd) error {
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
	return rawpatch.Patcher{
		Context: runCtx,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}.Patch(c.Raw, resolved)
}

func (c *imageInspectArtifactCmd) Run(ctx *runContext, parent *imageCmd) error {
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
	return artifactinspect.Inspector{Context: runCtx, Stdout: os.Stdout}.Artifact(resolved)
}

func (c *imageInspectOCICmd) Run(ctx *runContext, parent *imageCmd) error {
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
	return artifactinspect.Inspector{
		Context: runCtx,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}.OCI(resolved, c.Image)
}
