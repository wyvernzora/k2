package buildmetadata

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const DefaultPath = "/usr/share/k2/image-build/metadata.yaml"

// Read parses the flat key:value file produced by kairos/Dockerfile without
// adding a YAML dependency to the node agent.
func Read(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image build metadata: %w", err)
	}
	metadata := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key != "" {
			metadata[key] = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan image build metadata: %w", err)
	}
	return metadata, nil
}
