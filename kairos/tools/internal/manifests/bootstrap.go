package manifests

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/kairos/tools/internal/clusterconfig"
	"gopkg.in/yaml.v3"
)

type BootstrapOptions struct {
	OnePasswordTokenFile string
	SecretNamespace      string
	SecretName           string
	CiliumAPIHost        string
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

	secretNamespace := opts.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = "secrets"
	}
	addNamespace(&buf, secretNamespace)
	if opts.OnePasswordTokenFile != "" {
		secretName := opts.SecretName
		if secretName == "" {
			secretName = "onepassword-service-account-token"
		}
		if err := addBootstrapSecret(&buf, opts.OnePasswordTokenFile, secretNamespace, secretName); err != nil {
			return nil, err
		}
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

func addBootstrapSecret(buf *bytes.Buffer, tokenFile string, namespace string, name string) error {
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return fmt.Errorf("read 1Password service account token file %s: %w", tokenFile, err)
	}
	token = []byte(strings.TrimSpace(string(token)))
	if len(token) == 0 {
		return fmt.Errorf("1Password service account token file %s is empty", tokenFile)
	}
	separator(buf)
	fmt.Fprintf(buf, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s\n  namespace: %s\ntype: Opaque\ndata:\n  token: %s\n",
		name,
		namespace,
		base64.StdEncoding.EncodeToString(token),
	)
	return nil
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
