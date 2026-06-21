package toolcli

import (
	"context"

	"github.com/wyvernzora/k2/tools/internal/buildtool"
)

type buildCmd struct {
	CRDConstructs buildCRDConstructsCmd `cmd:"" name:"crd-constructs" help:"Generate TypeScript CRD bindings for all app-owned CRD manifests."`
	CRDManifest   buildCRDManifestCmd   `cmd:"" name:"crd-manifest" help:"Extract CRDs from one app's Helm chart dependencies."`
	Manifests     buildManifestsCmd     `cmd:"" help:"Synthesize deploy/ manifests."`
	DiffManifests buildDiffManifestsCmd `cmd:"" name:"diff-manifests" help:"Diff deploy/ against the remote deploy branch."`
	Lint          buildLintCmd          `cmd:"" help:"Run CRD generation, typecheck, eslint-rule tests, and eslint."`
}

type buildCRDConstructsCmd struct {
	OutputRoot string `name:"output-root" help:"Mirror generated bindings under this root instead of writing beside app CRD manifests." type:"path"`
	AppRoot    string `arg:"" optional:"" help:"Optional app directory. Defaults to all apps with crds/crds.k8s.yaml." type:"path"`
}

type buildCRDManifestCmd struct {
	AppRoot string `arg:"" help:"App directory, such as apps/longhorn." type:"path"`
}

type buildManifestsCmd struct{}

type buildDiffManifestsCmd struct {
	RemoteURL string `arg:"" optional:"" default:"https://github.com/wyvernzora/k2.git" help:"Git remote URL containing deploy."`
}

type buildLintCmd struct{}

func (c *buildCRDConstructsCmd) Run(ctx *runContext) error {
	runCtx, done := buildCommandContext()
	defer done()
	return buildtool.GenerateCRDConstructs(runCtx, buildtool.CRDConstructsOptions{
		Options:    buildOptions(ctx),
		OutputRoot: c.OutputRoot,
		AppRoot:    c.AppRoot,
	})
}

func (c *buildCRDManifestCmd) Run(ctx *runContext) error {
	runCtx, done := buildCommandContext()
	defer done()
	return buildtool.ExtractCRDManifest(runCtx, buildtool.CRDManifestOptions{
		Options: buildOptions(ctx),
		AppRoot: c.AppRoot,
	})
}

func (c *buildManifestsCmd) Run(ctx *runContext) error {
	runCtx, done := buildCommandContext()
	defer done()
	return buildtool.SynthesizeManifests(runCtx, buildOptions(ctx))
}

func (c *buildDiffManifestsCmd) Run(ctx *runContext) error {
	runCtx, done := buildCommandContext()
	defer done()
	return buildtool.DiffManifests(runCtx, buildtool.DiffManifestsOptions{
		Options:   buildOptions(ctx),
		RemoteURL: c.RemoteURL,
	})
}

func (c *buildLintCmd) Run(ctx *runContext) error {
	runCtx, done := buildCommandContext()
	defer done()
	return buildtool.Lint(runCtx, buildOptions(ctx))
}

func buildOptions(ctx *runContext) buildtool.Options {
	return buildtool.Options{
		RepoRoot: ctx.repoRoot,
		Jobs:     ctx.jobs,
		Reporter: reporter,
	}
}

func buildCommandContext() (runCtx context.Context, done func()) {
	runCtx, cancel := context.WithCancel(context.Background())
	reporter.SetInterruptCancel(cancel)
	return runCtx, func() {
		cancel()
		reporter.SetInterruptCancel(nil)
	}
}
