package build

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

var privateKeyDataKeys = map[string]struct{}{
	"ca.key":  {},
	"tls.key": {},
}

func validateNoEmbeddedPrivateKeys(manifest []byte) error {
	docs, err := parseYAMLDocuments(manifest)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if yamlKind(doc) != "Secret" {
			continue
		}
		for _, field := range []string{"data", "stringData"} {
			values := yamlMappingAt(doc, field)
			for key := range privateKeyDataKeys {
				if yamlMappingHasKey(values, key) {
					return fmt.Errorf("Secret %s embeds private key field %s.%s; generate private keys at runtime instead", yamlMetadataName(doc), field, key)
				}
			}
		}
	}
	return nil
}

func yamlMappingAt(doc *yaml.Node, path ...string) *yaml.Node {
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	for _, key := range path {
		if node.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			return nil
		}
		node = next
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	return node
}

func yamlMappingHasKey(node *yaml.Node, key string) bool {
	if node == nil {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}
