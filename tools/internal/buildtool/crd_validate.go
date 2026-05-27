package buildtool

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wyvernzora/k2/tools/internal/buildtool/dyffx"
	"gopkg.in/yaml.v3"
)

func validateRenderedCRDs(appRoot string, renderedCRDs []byte, crdExcludes []string) error {
	appName := filepath.Base(appRoot)
	tempDir, err := os.MkdirTemp("", "k2-"+appName+"-crds-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	renderedPath := filepath.Join(tempDir, "rendered-crds.k8s.yaml")
	if err := os.WriteFile(renderedPath, ensureTrailingNewline(renderedCRDs), 0o644); err != nil {
		return err
	}
	committedPath := filepath.Join(appRoot, "crds", "crds.k8s.yaml")
	if diff, different, err := dyffx.BetweenFiles(committedPath, renderedPath, crdExcludes); err != nil {
		return err
	} else if different {
		return fmt.Errorf("[%s] Helm chart CRDs differ from apps/%s/crds/crds.k8s.yaml\nRefusing to synthesize manifests so CRD upgrades can be reviewed and sequenced manually.\nRun: earthly +crd-manifest --APP_ROOT=apps/%s\n\n%s", appName, appName, appName, diff)
	}
	return nil
}

func parseCRDDocuments(data []byte) ([]*yaml.Node, error) {
	docs, err := parseYAMLDocuments(data)
	if err != nil {
		return nil, err
	}
	var crds []*yaml.Node
	for _, doc := range docs {
		if yamlKind(doc) == "CustomResourceDefinition" {
			crds = append(crds, doc)
		}
	}
	return crds, nil
}
