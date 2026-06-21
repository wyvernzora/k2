package buildtool

import (
	"io"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

const (
	dyffIgnorePath = ".dyffignore"
	deployBranch   = "deploy"
)

type appInfo struct {
	Name                 string `json:"name"`
	AppPath              string `json:"appPath"`
	DeployPath           string `json:"deployPath"`
	SourcePath           string `json:"sourcePath"`
	DestinationNamespace string `json:"destinationNamespace"`
}

type Options struct {
	RepoRoot string
	Jobs     int
	Reporter *ui.Reporter
}

type CRDConstructsOptions struct {
	Options
	OutputRoot string
	AppRoot    string
}

type CRDManifestOptions struct {
	Options
	AppRoot string
}

type DiffManifestsOptions struct {
	Options
	RemoteURL string
}

func (o Options) reporter() *ui.Reporter {
	if o.Reporter != nil {
		return o.Reporter
	}
	return ui.New(io.Discard, true)
}
