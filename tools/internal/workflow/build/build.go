package build

func buildRegistration() Registration {
	return Registration{Name: "build", Help: "Run K2 build, synth, lint, and diff workflows.", Order: 70, Command: &buildCmd{}}
}

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
