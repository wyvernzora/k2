package keys

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var allowedPrefixes = []string{
	"ssh-ed25519 ",
}

func Load(literals []string, files []string) ([]string, error) {
	var raw []string
	raw = append(raw, literals...)
	for _, path := range files {
		keys, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		raw = append(raw, keys...)
	}

	seen := map[string]bool{}
	var out []string
	for _, key := range raw {
		key = strings.TrimSpace(key)
		if key == "" || strings.HasPrefix(key, "#") {
			continue
		}
		if err := validate(key); err != nil {
			return nil, err
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one literal operator SSH public key is required")
	}
	return out, nil
}

func loadFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read operator key file %s: %w", path, err)
	}
	defer f.Close()

	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read operator key file %s: %w", path, err)
	}
	return out, nil
}

func validate(key string) error {
	if strings.HasPrefix(key, "github:") {
		return fmt.Errorf("github: operator keys are not allowed; pass literal public keys instead")
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(key, prefix) {
			fields := strings.Fields(key)
			if len(fields) < 2 {
				return fmt.Errorf("operator key %q is missing public key material", key)
			}
			return nil
		}
	}
	return fmt.Errorf("operator key %q must be a literal ssh-ed25519 public key", key)
}
