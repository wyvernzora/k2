package dyffx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"gopkg.in/yaml.v3"
)

// BetweenFiles compares two YAML/JSON files using dyff's semantic
// model and returns human-readable output when there are differences.
func BetweenFiles(fromPath, toPath string, excludes []string) (output string, different bool, err error) {
	if len(excludes) > 0 {
		tempDir, err := os.MkdirTemp("", "k2-dyff-*")
		if err != nil {
			return "", false, err
		}
		defer os.RemoveAll(tempDir)

		normalizedFrom := filepath.Join(tempDir, "from.yaml")
		normalizedTo := filepath.Join(tempDir, "to.yaml")
		if err := writeNormalizedYAML(fromPath, normalizedFrom, excludes); err != nil {
			return "", false, err
		}
		if err := writeNormalizedYAML(toPath, normalizedTo, excludes); err != nil {
			return "", false, err
		}
		fromPath = normalizedFrom
		toPath = normalizedTo
	}

	from, to, err := ytbx.LoadFiles(fromPath, toPath)
	if err != nil {
		return "", false, fmt.Errorf("load dyff inputs: %w", err)
	}
	report, err := dyff.CompareInputFiles(from, to,
		dyff.IgnoreOrderChanges(false),
		dyff.IgnoreWhitespaceChanges(false),
		dyff.KubernetesEntityDetection(true),
	)
	if err != nil {
		return "", false, fmt.Errorf("compare dyff inputs: %w", err)
	}
	if len(report.Diffs) == 0 {
		return "", false, nil
	}

	var buf bytes.Buffer
	human := dyff.HumanReport{
		Report:          report,
		Indent:          2,
		UseIndentLines:  true,
		OmitHeader:      true,
		UseGoPatchPaths: false,
	}
	if err := human.WriteReport(&buf); err != nil {
		return "", false, err
	}
	return strings.TrimLeft(buf.String(), "\n"), true, nil
}

func writeNormalizedYAML(srcPath, dstPath string, excludes []string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	docs, err := parseYAMLDocuments(data)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		for _, exclude := range excludes {
			if err := pruneYAMLPath(doc, exclude); err != nil {
				return err
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			_ = enc.Close()
			return err
		}
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(dstPath, ensureTrailingNewline(buf.Bytes()), 0o644)
}

func parseYAMLDocuments(data []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc.Content) == 0 {
			continue
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

func pruneYAMLPath(doc *yaml.Node, path string) error {
	parsed, err := ytbx.ParsePathStringUnsafe(path)
	if err != nil {
		return err
	}
	removeYAMLPath(doc, parsed.PathElements)
	return nil
}

func removeYAMLPath(node *yaml.Node, path []ytbx.PathElement) bool {
	if node == nil || len(path) == 0 {
		return false
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return removeYAMLPath(node.Content[0], path)
	}
	head := path[0]
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value != head.Name {
				continue
			}
			if len(path) == 1 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return true
			}
			return removeYAMLPath(node.Content[i+1], path[1:])
		}
	case yaml.SequenceNode:
		if head.Name != "" || head.Idx < 0 || head.Idx >= len(node.Content) {
			return false
		}
		if len(path) == 1 {
			node.Content = append(node.Content[:head.Idx], node.Content[head.Idx+1:]...)
			return true
		}
		return removeYAMLPath(node.Content[head.Idx], path[1:])
	}
	return false
}

func ensureTrailingNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	return append(data, '\n')
}
