package manifests

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/kairos/tools/internal/clusterconfig"
	"gopkg.in/yaml.v3"
)

type BootstrapOptions struct {
	ExtraManifestPatterns []string
	CiliumAPIHost         string
}

func Bootstrap(repoRoot string, cfg clusterconfig.Config, opts BootstrapOptions) ([]byte, error) {
	deployDir := cfg.DeployDir(repoRoot)
	var buf bytes.Buffer

	addNamespace(&buf, "cilium")
	if err := appendFile(&buf, filepath.Join(deployDir, "apps", "cilium", "crds.k8s.yaml")); err != nil {
		return nil, err
	}
	if err := appendCiliumApp(&buf, filepath.Join(deployDir, "apps", "cilium", "app.k8s.yaml"), opts.CiliumAPIHost); err != nil {
		return nil, err
	}

	addNamespace(&buf, "argocd")
	if err := appendFile(&buf, filepath.Join(deployDir, "apps", "argocd", "crds.k8s.yaml")); err != nil {
		return nil, err
	}
	if err := appendFile(&buf, filepath.Join(deployDir, "apps", "argocd", "app.k8s.yaml")); err != nil {
		return nil, err
	}

	addNamespace(&buf, "kube-vip")
	if err := appendFile(&buf, filepath.Join(deployDir, "apps", "kube-vip", "app.k8s.yaml")); err != nil {
		return nil, err
	}

	extraManifestPaths, err := expandExtraManifestPatterns(opts.ExtraManifestPatterns)
	if err != nil {
		return nil, err
	}
	if err := appendExtraManifests(&buf, extraManifestPaths, map[string]bool{
		"argocd":   true,
		"cilium":   true,
		"kube-vip": true,
	}); err != nil {
		return nil, err
	}

	if err := appendFile(&buf, filepath.Join(deployDir, "argocd", "app.k8s.yaml")); err != nil {
		return nil, err
	}
	addCleanupJob(&buf)

	return buf.Bytes(), nil
}

func addNamespace(buf *bytes.Buffer, name string) {
	separator(buf)
	fmt.Fprintf(buf, "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n", name)
}

func expandExtraManifestPatterns(patterns []string) ([]string, error) {
	var paths []string
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if strings.ContainsAny(pattern, "*?[") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("expand extra manifests glob %s: %w", pattern, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("extra manifests glob %s did not match any files", pattern)
			}
			sort.Strings(matches)
			paths = append(paths, matches...)
			continue
		}
		if _, err := os.Stat(pattern); err != nil {
			return nil, fmt.Errorf("stat extra manifest %s: %w", pattern, err)
		}
		paths = append(paths, pattern)
	}
	return paths, nil
}

func appendExtraManifests(buf *bytes.Buffer, paths []string, existingNamespaces map[string]bool) error {
	if len(paths) == 0 {
		return nil
	}

	requiredNamespaces := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read extra manifest %s: %w", path, err)
		}
		namespaces, err := manifestNamespaces(data, path)
		if err != nil {
			return err
		}
		for namespace := range namespaces {
			requiredNamespaces[namespace] = true
		}
	}

	namespaceNames := make([]string, 0, len(requiredNamespaces))
	for namespace := range requiredNamespaces {
		if existingNamespaces[namespace] {
			continue
		}
		namespaceNames = append(namespaceNames, namespace)
	}
	sort.Strings(namespaceNames)
	for _, namespace := range namespaceNames {
		addNamespace(buf, namespace)
	}

	for _, path := range paths {
		if err := appendFile(buf, path); err != nil {
			return err
		}
	}
	return nil
}

func manifestNamespaces(data []byte, path string) (map[string]bool, error) {
	required := map[string]bool{}
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
		kind := mappingValue(root, "kind")
		metadata := mappingNodeValue(root, "metadata")
		if metadata == nil || metadata.Kind != yaml.MappingNode {
			continue
		}
		if kind == "Namespace" {
			continue
		}
		if namespace := mappingValue(metadata, "namespace"); namespace != "" {
			required[namespace] = true
		}
	}
	return required, nil
}

func addCleanupJob(buf *bytes.Buffer) {
	separator(buf)
	buf.WriteString(`apiVersion: batch/v1
kind: Job
metadata:
  name: k2-bootstrap-manifest-cleanup
  namespace: kube-system
spec:
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: OnFailure
      nodeSelector:
        node-role.kubernetes.io/control-plane: "true"
      tolerations:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
      containers:
        - name: cleanup
          image: busybox:1.37
          command:
            - sh
            - -c
            - rm -f /host-manifests/k2-bootstrap.yaml
          volumeMounts:
            - name: manifests
              mountPath: /host-manifests
      volumes:
        - name: manifests
          hostPath:
            path: /var/lib/rancher/k3s/server/manifests
            type: Directory
`)
}

func appendFile(buf *bytes.Buffer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read bootstrap manifest %s: %w", path, err)
	}
	data = trimLeadingDocumentSeparator(data)
	separator(buf)
	buf.Write(data)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return nil
}

func appendCiliumApp(buf *bytes.Buffer, path string, apiHost string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read bootstrap manifest %s: %w", path, err)
	}
	if apiHost != "" {
		data, err = patchKubernetesServiceHost(data, apiHost)
		if err != nil {
			return fmt.Errorf("patch Cilium Kubernetes API host in %s: %w", path, err)
		}
	}
	data = trimLeadingDocumentSeparator(data)
	separator(buf)
	buf.Write(data)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return nil
}

func patchKubernetesServiceHost(data []byte, apiHost string) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	patches := 0
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		patches += patchKubernetesServiceHostNode(&doc, apiHost)
		docs = append(docs, &doc)
	}
	if patches == 0 {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST env var not found")
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	for _, doc := range docs {
		if err := encoder.Encode(doc); err != nil {
			_ = encoder.Close()
			return nil, err
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func patchKubernetesServiceHostNode(node *yaml.Node, apiHost string) int {
	if node == nil {
		return 0
	}
	patches := 0
	if node.Kind == yaml.MappingNode && mappingValue(node, "name") == "KUBERNETES_SERVICE_HOST" {
		if value := mappingNodeValue(node, "value"); value != nil {
			value.Kind = yaml.ScalarNode
			value.Tag = "!!str"
			value.Value = apiHost
			patches++
		}
	}
	for _, child := range node.Content {
		patches += patchKubernetesServiceHostNode(child, apiHost)
	}
	return patches
}

func mappingValue(node *yaml.Node, key string) string {
	value := mappingNodeValue(node, key)
	if value == nil {
		return ""
	}
	return value.Value
}

func mappingNodeValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func trimLeadingDocumentSeparator(data []byte) []byte {
	data = bytes.TrimLeft(data, "\r\n\t ")
	data = bytes.TrimPrefix(data, []byte("---\n"))
	return bytes.TrimPrefix(data, []byte("---\r\n"))
}

func separator(buf *bytes.Buffer) {
	if buf.Len() > 0 {
		buf.WriteString("---\n")
	}
}
