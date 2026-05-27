package toolcli

import (
	"os"

	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/config"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/paths"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
)

type imageCmd struct {
	BuildRoot    string `name:"build-root" help:"Kairos image build input root." type:"path"`
	KairosRoot   string `name:"kairos-root" help:"Kairos image configuration root." type:"path"`
	TargetsFile  string `name:"targets" help:"targets.yaml path." type:"path"`
	VersionsFile string `name:"versions" help:"versions.env path." type:"path"`
	OverlaysDir  string `name:"overlays" help:"overlays directory." type:"path"`
	ArtifactsDir string `name:"artifacts" help:"artifacts directory." type:"path"`

	Build   imageBuildCmd   `cmd:"" help:"Build Kairos image artifacts."`
	Plan    imagePlanCmd    `cmd:"" help:"Print resolved target plans."`
	Patch   imagePatchCmd   `cmd:"" help:"Patch generated Kairos image artifacts."`
	Inspect imageInspectCmd `cmd:"" help:"Inspect generated artifacts."`
}

type imageGlobals struct {
	buildRoot    string
	kairosRoot   string
	targetsFile  string
	versionsFile string
	overlaysDir  string
	artifactsDir string
}

type imageBuildCmd struct {
	Artifact imageBuildArtifactCmd `cmd:"" help:"Build bootable artifacts for a target."`
	OCI      imageBuildOCICmd      `cmd:"" name:"oci" help:"Build OCI images for targets."`
}

type imageBuildArtifactCmd struct {
	Target string `arg:"" help:"Target name."`
}

type imageBuildOCICmd struct {
	Push      bool   `help:"Push images instead of loading them locally."`
	NoCache   bool   `name:"no-cache" help:"Build without Docker layer cache."`
	CacheFrom string `name:"cache-from" help:"Forward a docker buildx --cache-from value."`
	CacheTo   string `name:"cache-to" help:"Forward a docker buildx --cache-to value."`
	All       bool   `help:"Build all enabled targets."`
	Target    string `arg:"" optional:"" help:"Target name."`
}

type imagePlanCmd struct {
	Format string `default:"yaml" enum:"yaml,json" help:"Output format."`
	All    bool   `help:"Print plans for all enabled targets."`
	Target string `arg:"" optional:"" help:"Target name."`
}

type imageInspectCmd struct {
	Artifact imageInspectArtifactCmd `cmd:"" help:"Inspect bootable artifacts for a target."`
	OCI      imageInspectOCICmd      `cmd:"" name:"oci" help:"Inspect a built OCI image for a target."`
}

type imagePatchCmd struct {
	Raw imagePatchRawCmd `cmd:"" help:"Patch a generated raw artifact."`
}

type imagePatchRawCmd struct {
	Target string `arg:"" help:"Target name."`
	Raw    string `name:"raw" required:"" help:"Raw artifact path." type:"path"`
}

type imageInspectArtifactCmd struct {
	Target string `arg:"" help:"Target name."`
}

type imageInspectOCICmd struct {
	Image  string `name:"image" help:"Image tag to inspect instead of the resolved target image."`
	Target string `arg:"" help:"Target name."`
}

type imagePlansOutput struct {
	Targets []plan.Plan `json:"targets" yaml:"targets"`
}

func (c imageCmd) imageGlobals() imageGlobals {
	return imageGlobals{
		buildRoot:    c.BuildRoot,
		kairosRoot:   c.KairosRoot,
		targetsFile:  c.TargetsFile,
		versionsFile: c.VersionsFile,
		overlaysDir:  c.OverlaysDir,
		artifactsDir: c.ArtifactsDir,
	}
}

func loadImagePlanner(globals imageGlobals) (plan.Planner, error) {
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
