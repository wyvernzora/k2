package build

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	allowCRDTemplateFailureAnnotation = "k2.wyvernzora.io/allow-crd-template-failure"
	allowCRDEmptyRenderAnnotation     = "k2.wyvernzora.io/allow-crd-empty-render"
)

type chartYAML struct {
	Annotations  map[string]string `yaml:"annotations"`
	Dependencies []struct {
		Repository string `yaml:"repository"`
	} `yaml:"dependencies"`
}

func chartExists(appRoot string) bool {
	_, err := os.Stat(filepath.Join(appRoot, "Chart.yaml"))
	return err == nil
}

func readChart(appRoot string) (chartYAML, error) {
	data, err := os.ReadFile(filepath.Join(appRoot, "Chart.yaml"))
	if err != nil {
		return chartYAML{}, err
	}
	var chart chartYAML
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return chartYAML{}, err
	}
	return chart, nil
}

func (c chartYAML) Repositories() []string {
	seen := map[string]bool{}
	for _, dep := range c.Dependencies {
		repo := strings.TrimSpace(dep.Repository)
		if repo != "" {
			seen[repo] = true
		}
	}
	repos := make([]string, 0, len(seen))
	for repo := range seen {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	return repos
}

func (c chartYAML) AllowCRDTemplateFailure() bool {
	return strings.EqualFold(strings.TrimSpace(c.Annotations[allowCRDTemplateFailureAnnotation]), "true")
}

func (c chartYAML) AllowCRDEmptyRender() bool {
	return strings.EqualFold(strings.TrimSpace(c.Annotations[allowCRDEmptyRenderAnnotation]), "true")
}
