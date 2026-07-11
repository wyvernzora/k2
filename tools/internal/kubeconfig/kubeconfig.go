package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func CredentialsDir(clusterName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "k2", clusterName), nil
}

func Path(clusterName string) (string, error) {
	dir, err := CredentialsDir(clusterName)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "kubeconfig")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("kubeconfig not found at %s; run `k2-tools provision bootstrap` first or pass --cluster-name <existing>", path)
	}
	return path, nil
}

func RewriteServer(data []byte, server string) ([]byte, error) {
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}

	clusters, ok := cfg["clusters"].([]any)
	if !ok || len(clusters) == 0 {
		return nil, fmt.Errorf("kubeconfig has no clusters")
	}

	for _, item := range clusters {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("kubeconfig cluster entry has unexpected shape")
		}
		cluster, ok := entry["cluster"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("kubeconfig cluster entry is missing cluster data")
		}
		cluster["server"] = server
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("render kubeconfig: %w", err)
	}
	return out, nil
}
