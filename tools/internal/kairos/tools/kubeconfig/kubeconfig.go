package kubeconfig

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
