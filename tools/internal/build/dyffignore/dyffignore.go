package dyffignore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gonvenience/ytbx"
)

// Rules is a sectioned dyff ignore file. Sections are lower-cased names such
// as "app" and "crd"; entries are ytbx GoPatch-style YAML paths. They are
// pruned as subtrees before dyff compares the inputs.
type Rules map[string][]string

// Load reads a sectioned dyffignore file. A missing file returns an
// empty rule set.
func Load(path string) (Rules, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Rules{}, nil
		}
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

// Parse parses a sectioned dyffignore stream.
func Parse(r io.Reader) (Rules, error) {
	scanner := bufio.NewScanner(r)
	rules := Rules{}
	section := ""
	for lineNo := 1; scanner.Scan(); lineNo++ {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]")))
			if section == "" {
				return nil, fmt.Errorf("line %d: empty section name", lineNo)
			}
			if _, ok := rules[section]; !ok {
				rules[section] = nil
			}
			continue
		}
		if section == "" {
			return nil, fmt.Errorf("line %d: ignore path %q appears before any section", lineNo, raw)
		}
		if !strings.HasPrefix(raw, "/") {
			return nil, fmt.Errorf("line %d: ignore path %q must start with /", lineNo, raw)
		}
		path, err := ytbx.ParsePathStringUnsafe(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid ignore path %q: %w", lineNo, raw, err)
		}
		for _, element := range path.PathElements {
			if element.Key != "" {
				return nil, fmt.Errorf("line %d: ignore path %q uses named-list selectors, which .dyffignore does not support", lineNo, raw)
			}
		}
		rules[section] = append(rules[section], raw)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

// Section returns a copy of the ignore paths for name.
func (r Rules) Section(name string) []string {
	entries := r[strings.ToLower(name)]
	return append([]string(nil), entries...)
}
