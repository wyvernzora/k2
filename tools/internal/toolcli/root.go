package toolcli

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/workspace"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

type cli struct {
	RepoRoot string `name:"repo-root" env:"K2_TOOLS_REPO_ROOT" help:"Repository root. Defaults to auto-detection." type:"path"`
	Plain    bool   `name:"plain" env:"K2_TOOLS_PLAIN" help:"Use plain log output without grouped status markers."`
	Jobs     int    `name:"jobs" env:"K2_TOOLS_JOBS" default:"4" help:"Maximum concurrent jobs for build-style operations."`

	Provision provisionCmd `cmd:"" help:"Provision Kairos-backed K3s nodes."`
	VM        vmCmd        `cmd:"" help:"Manage local test VMs."`
	Flash     flashCmd     `cmd:"" help:"Flash Kairos images to hardware."`
	Upgrade   upgradeCmd   `cmd:"" help:"Upgrade a Kairos node's image in place."`
	Image     imageCmd     `cmd:"" help:"Plan, build, patch, and inspect Kairos images."`
	Build     buildCmd     `cmd:"" help:"Run K2 build, synth, lint, and diff workflows."`
}

type runContext struct {
	repoRoot string
	jobs     int
}

var reporter = ui.New(os.Stderr, false)

func Main(name string, args []string) int {
	if err := runWithName(name, args); err != nil {
		reporter.Errorf("%v", err)
		return 1
	}
	return 0
}

func run(args []string) error {
	return runWithName("k2-tools", args)
}

func runWithName(name string, args []string) error {
	var app cli
	parser, err := kong.New(&app, kong.Name(name), kong.UsageOnError())
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	reporter = ui.New(os.Stderr, app.Plain)
	repoRoot, err := workspace.FindRepoRoot(app.RepoRoot)
	if err != nil {
		return err
	}
	return ctx.Run(&runContext{repoRoot: repoRoot, jobs: app.Jobs}, &app.Image)
}
