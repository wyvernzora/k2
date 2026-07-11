package build

const (
	dyffIgnorePath = ".dyffignore"
	deployBranch   = "deploy"
)

type AppInfo struct {
	Name                 string `json:"name"`
	AppPath              string `json:"appPath"`
	DeployPath           string `json:"deployPath"`
	SourcePath           string `json:"sourcePath"`
	DestinationNamespace string `json:"destinationNamespace"`
}

type Options struct {
	RepoRoot string
	Jobs     int
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
