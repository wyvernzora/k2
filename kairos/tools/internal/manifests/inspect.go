package manifests

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ExtraManifestObject is one Kubernetes object listed in an
// extra-manifest YAML file. Multi-doc YAML files produce one
// ExtraManifestObject per document. Used by the provision plan
// renderer to surface what's about to be applied without dumping
// the verbatim manifests.
type ExtraManifestObject struct {
	Path       string
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// InspectExtraManifests expands `patterns` (file paths or globs)
// and returns the Kubernetes objects found. Skips documents that
// don't carry a kind + name (the same lenient parse the bundle
// assembler uses).
//
// Order is stable: paths are sorted lexically (matching the
// expander used by Bootstrap()), and within each file the documents
// appear in declaration order.
func InspectExtraManifests(patterns []string) ([]ExtraManifestObject, error) {
	paths, err := expandExtraManifestPatterns(patterns)
	if err != nil {
		return nil, err
	}
	var out []ExtraManifestObject
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read extra manifest %s: %w", path, err)
		}
		objs, err := manifestObjects(data, path)
		if err != nil {
			return nil, err
		}
		out = append(out, objs...)
	}
	return out, nil
}

func manifestObjects(data []byte, path string) ([]ExtraManifestObject, error) {
	var out []ExtraManifestObject
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse extra manifest %s: %w", path, err)
		}
		if doc.Kind == 0 || len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			continue
		}
		apiVersion := mappingValue(root, "apiVersion")
		kind := mappingValue(root, "kind")
		if apiVersion == "" || kind == "" {
			continue
		}
		var name, namespace string
		if md := mappingNodeValue(root, "metadata"); md != nil && md.Kind == yaml.MappingNode {
			name = mappingValue(md, "name")
			namespace = mappingValue(md, "namespace")
		}
		if name == "" {
			continue
		}
		out = append(out, ExtraManifestObject{
			Path:       path,
			APIVersion: apiVersion,
			Kind:       kind,
			Namespace:  namespace,
			Name:       name,
		})
	}
	return out, nil
}
