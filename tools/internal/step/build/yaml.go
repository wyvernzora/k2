package build

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

func parseYAMLDocuments(data []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
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

func encodeYAMLDocuments(docs []*yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			_ = enc.Close()
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return ensureTrailingNewline(buf.Bytes()), nil
}

func yamlKind(doc *yaml.Node) string {
	return yamlScalarAt(doc, "kind")
}

func yamlMetadataName(doc *yaml.Node) string {
	if name := yamlScalarAt(doc, "metadata", "name"); name != "" {
		return name
	}
	return "(unnamed)"
}

func yamlScalarAt(doc *yaml.Node, path ...string) string {
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	for _, key := range path {
		if node.Kind != yaml.MappingNode {
			return ""
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			return ""
		}
		node = next
	}
	if node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func ensureTrailingNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	out := append([]byte(nil), data...)
	return append(out, '\n')
}
