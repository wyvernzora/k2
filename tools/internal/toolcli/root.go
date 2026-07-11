package toolcli

import (
	"os"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/tools/internal/ui"
	"github.com/wyvernzora/k2/tools/internal/workflow"
	workflowbuild "github.com/wyvernzora/k2/tools/internal/workflow/build"
	workflowe2e "github.com/wyvernzora/k2/tools/internal/workflow/e2e"
	workflowimage "github.com/wyvernzora/k2/tools/internal/workflow/image"
	workflowprovision "github.com/wyvernzora/k2/tools/internal/workflow/provision"
	workflowupgrade "github.com/wyvernzora/k2/tools/internal/workflow/upgrade"
	workflowvm "github.com/wyvernzora/k2/tools/internal/workflow/vm"
	"github.com/wyvernzora/k2/tools/internal/workspace"
)

type cli struct {
	RepoRoot string `name:"repo-root" env:"K2_TOOLS_REPO_ROOT" help:"Repository root. Defaults to auto-detection." type:"path"`
	Plain    bool   `name:"plain" env:"K2_TOOLS_PLAIN" help:"Use plain log output without grouped status markers."`
	Jobs     int    `name:"jobs" env:"K2_TOOLS_JOBS" default:"4" help:"Maximum concurrent jobs for build-style operations."`
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
	registrations := builtinRegistrations()
	options := []kong.Option{kong.Name(name), kong.UsageOnError()}
	bindings := make([]any, 0, len(registrations)+1)
	for _, registration := range registrations {
		options = append(options, kong.DynamicCommand(
			registration.Name,
			registration.Help,
			"",
			registration.Command,
			registrationTags(registration)...,
		))
		bindings = append(bindings, registration.Command)
	}

	parser, err := kong.New(&app, options...)
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	reporter = ui.New(os.Stderr, app.Plain)
	workflow.SetReporter(reporter)
	repoRoot, err := workspace.FindRepoRoot(app.RepoRoot)
	if err != nil {
		return err
	}
	bindings = append(bindings, workflow.NewRuntime(repoRoot, app.Jobs))
	return ctx.Run(bindings...)
}

func builtinRegistrations() []workflow.Registration {
	registrations := append([]workflow.Registration{}, workflowprovision.Registrations()...)
	registrations = append(registrations, workflowe2e.Registrations()...)
	registrations = append(registrations, workflowupgrade.Registrations()...)
	registrations = append(registrations, workflowimage.Registrations()...)
	registrations = append(registrations, workflowvm.Registrations()...)
	registrations = append(registrations, workflowbuild.Registrations()...)
	sort.SliceStable(registrations, func(i, j int) bool {
		return registrations[i].Order < registrations[j].Order
	})
	return registrations
}

func registrationTags(registration workflow.Registration) []string {
	if len(registration.Aliases) == 0 {
		return nil
	}
	return []string{`aliases:"` + strings.Join(registration.Aliases, ",") + `"`}
}
