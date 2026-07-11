package image

import (
	"context"
	"os"

	stepimage "github.com/wyvernzora/k2/tools/internal/step/image"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func (c *imageBuildArtifactCmd) Run(ctx *Runtime, parent *imageCmd) error {
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
	wf := ui.NewWorkflow(currentReporter())
	wf.Task("Build artifact "+c.Target, func(ctx context.Context) error {
		return stepimage.ArtifactBuilder{Context: ctx, Stdout: os.Stdout, Stderr: os.Stderr}.Artifact(resolved)
	})
	return wf.Execute(runCtx)
}
