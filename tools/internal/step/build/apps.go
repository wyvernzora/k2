package build

import (
	"os"
	"path/filepath"
	"sort"
)

func discoverApps(repoRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(repoRoot, "apps"))
	if err != nil {
		return nil, err
	}
	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appRoot := filepath.Join(repoRoot, "apps", entry.Name())
		if _, err := os.Stat(filepath.Join(appRoot, "index.ts")); err == nil {
			apps = append(apps, appRoot)
		}
	}
	sort.Strings(apps)
	return apps, nil
}

func makeAppInfo(appName string) AppInfo {
	return AppInfo{
		Name:                 appName,
		AppPath:              filepath.Join("apps", appName),
		DeployPath:           filepath.Join("deploy", appName),
		SourcePath:           appName,
		DestinationNamespace: appName,
	}
}
