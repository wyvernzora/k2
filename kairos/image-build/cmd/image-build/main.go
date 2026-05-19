package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	artifactbuild "github.com/wyvernzora/k2/kairos/image-build/internal/artifact"
	"github.com/wyvernzora/k2/kairos/image-build/internal/config"
	artifactinspect "github.com/wyvernzora/k2/kairos/image-build/internal/inspect"
	ocibuild "github.com/wyvernzora/k2/kairos/image-build/internal/oci"
	"github.com/wyvernzora/k2/kairos/image-build/internal/paths"
	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
	"github.com/wyvernzora/k2/kairos/image-build/internal/rawpatch"
	"gopkg.in/yaml.v3"
)

type cli struct {
	BuildRoot    string `name:"build-root" help:"image-build tooling root." type:"path"`
	KairosRoot   string `name:"kairos-root" help:"Kairos image configuration root." type:"path"`
	TargetsFile  string `name:"targets" help:"targets.yaml path." type:"path"`
	VersionsFile string `name:"versions" help:"versions.env path." type:"path"`
	OverlaysDir  string `name:"overlays" help:"overlays directory." type:"path"`
	ArtifactsDir string `name:"artifacts" help:"artifacts directory." type:"path"`

	Build   buildCmd   `cmd:"" help:"Build Kairos image artifacts."`
	Plan    planCmd    `cmd:"" help:"Print resolved target plans."`
	Patch   patchCmd   `cmd:"" help:"Patch generated Kairos image artifacts."`
	Inspect inspectCmd `cmd:"" help:"Inspect generated artifacts."`
}

type globalFlags struct {
	buildRoot    string
	kairosRoot   string
	targetsFile  string
	versionsFile string
	overlaysDir  string
	artifactsDir string
}

type runContext struct {
	globals globalFlags
}

type buildCmd struct {
	Artifact buildArtifactCmd `cmd:"" help:"Build bootable artifacts for a target."`
	OCI      buildOCICmd      `cmd:"" name:"oci" help:"Build OCI images for targets."`
}

type buildArtifactCmd struct {
	Target string `arg:"" help:"Target name."`
}

type buildOCICmd struct {
	Push      bool   `help:"Push images instead of loading them locally."`
	NoCache   bool   `name:"no-cache" help:"Build without Docker layer cache."`
	CacheFrom string `name:"cache-from" help:"Forward a docker buildx --cache-from value."`
	CacheTo   string `name:"cache-to" help:"Forward a docker buildx --cache-to value."`
	All       bool   `help:"Build all enabled targets."`
	Target    string `arg:"" optional:"" help:"Target name."`
}

type planCmd struct {
	Format string `default:"yaml" enum:"yaml,json" help:"Output format."`
	All    bool   `help:"Print plans for all enabled targets."`
	Target string `arg:"" optional:"" help:"Target name."`
}

type inspectCmd struct {
	Artifact inspectArtifactCmd `cmd:"" help:"Inspect bootable artifacts for a target."`
	OCI      inspectOCICmd      `cmd:"" name:"oci" help:"Inspect a built OCI image for a target."`
}

type patchCmd struct {
	Raw patchRawCmd `cmd:"" help:"Patch a generated raw artifact."`
}

type patchRawCmd struct {
	Target string `arg:"" help:"Target name."`
	Raw    string `name:"raw" required:"" help:"Raw artifact path." type:"path"`
}

type inspectArtifactCmd struct {
	Target string `arg:"" help:"Target name."`
}

type inspectOCICmd struct {
	Image  string `name:"image" help:"Image tag to inspect instead of the resolved target image."`
	Target string `arg:"" help:"Target name."`
}

type plansOutput struct {
	Targets []plan.Plan `json:"targets" yaml:"targets"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var app cli
	parser, err := kong.New(
		&app,
		kong.Name("k2-image-build"),
		kong.Description("Build and inspect K2 Kairos images."),
		kong.UsageOnError(),
	)
	if err != nil {
		return err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run(&runContext{globals: app.globalFlags()})
}

func (c *buildArtifactCmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}

	return artifactbuild.Builder{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}.Artifact(resolved)
}

func (c *buildOCICmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}

	var plans []plan.Plan
	if c.All {
		if c.Target != "" {
			return fmt.Errorf("build oci --all does not accept a target argument")
		}
		plans, err = planner.BuildAllEnabled()
	} else {
		if c.Target == "" {
			return fmt.Errorf("build oci expects exactly one target or --all")
		}
		var resolved plan.Plan
		resolved, err = planner.Build(c.Target)
		plans = []plan.Plan{resolved}
	}
	if err != nil {
		return err
	}

	builder := ocibuild.Builder{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	options := ocibuild.Options{
		Push:      c.Push,
		NoCache:   c.NoCache,
		CacheFrom: c.CacheFrom,
		CacheTo:   c.CacheTo,
	}
	for _, resolved := range plans {
		if err := builder.Image(resolved, options); err != nil {
			return err
		}
	}
	return nil
}

func (c *planCmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}

	if c.All {
		if c.Target != "" {
			return fmt.Errorf("plan --all does not accept a target argument")
		}
		plans, err := planner.BuildAllEnabled()
		if err != nil {
			return err
		}
		return writeOutput(c.Format, plansOutput{Targets: plans})
	}

	if c.Target == "" {
		return fmt.Errorf("plan expects exactly one target or --all")
	}

	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}
	return writeOutput(c.Format, resolved)
}

func (c *patchRawCmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}

	return rawpatch.Patcher{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}.Patch(c.Raw, resolved)
}

func (c *inspectArtifactCmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}

	return artifactinspect.Inspector{Stdout: os.Stdout}.Artifact(resolved)
}

func (c *inspectOCICmd) Run(ctx *runContext) error {
	planner, err := loadPlanner(ctx.globals)
	if err != nil {
		return err
	}
	resolved, err := planner.Build(c.Target)
	if err != nil {
		return err
	}

	return artifactinspect.Inspector{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}.OCI(resolved, c.Image)
}

func (c cli) globalFlags() globalFlags {
	return globalFlags{
		buildRoot:    c.BuildRoot,
		kairosRoot:   c.KairosRoot,
		targetsFile:  c.TargetsFile,
		versionsFile: c.VersionsFile,
		overlaysDir:  c.OverlaysDir,
		artifactsDir: c.ArtifactsDir,
	}
}

func loadPlanner(globals globalFlags) (plan.Planner, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return plan.Planner{}, err
	}

	discovered, err := paths.Discover(cwd, paths.Overrides{
		BuildRoot:    globals.buildRoot,
		KairosRoot:   globals.kairosRoot,
		TargetsFile:  globals.targetsFile,
		VersionsFile: globals.versionsFile,
		OverlaysDir:  globals.overlaysDir,
		ArtifactsDir: globals.artifactsDir,
	})
	if err != nil {
		return plan.Planner{}, err
	}

	targets, err := config.LoadTargets(discovered.TargetsFile)
	if err != nil {
		return plan.Planner{}, err
	}
	versions, err := config.LoadVersions(discovered.VersionsFile)
	if err != nil {
		return plan.Planner{}, err
	}

	return plan.New(targets, versions, discovered), nil
}

func writeOutput(format string, value any) error {
	switch format {
	case "json":
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
	case "yaml":
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		fmt.Print(string(encoded))
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	return nil
}
