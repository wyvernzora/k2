package toolcli

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

const (
	e2eStoragePreset = "qemu-vmnet-storage"
	e2eK8sPreset     = "qemu-vmnet"
	e2ePVCName       = "e2e-storage-pvc"
	e2ePodName       = "e2e-storage-pod"
)

type e2eVMNames struct {
	Storage string
	Server  string
	Workers []string
}

func (c e2eStorageCmd) defaults() e2eStorageCmd {
	if c.Workers == 0 {
		c.Workers = 1
	}
	if c.ClusterTarget == "" {
		c.ClusterTarget = "v3"
	}
	if c.ClusterName == "" {
		c.ClusterName = "k2e2e"
	}
	if c.Namespace == "" {
		c.Namespace = "e2e-storage"
	}
	if c.PVCSize == "" {
		c.PVCSize = "1Gi"
	}
	return c
}

func e2eStorageVMNames(clusterName string, workers int) e2eVMNames {
	base := sanitizeE2EName(clusterName)
	names := e2eVMNames{
		Storage: "e2e-" + base + "-storage",
		Server:  "e2e-" + base + "-server",
	}
	for i := range workers {
		names.Workers = append(names.Workers, fmt.Sprintf("e2e-%s-worker-%d", base, i+1))
	}
	return names
}

func sanitizeE2EName(value string) string {
	value = strings.ToLower(value)
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "k2e2e"
	}
	return value
}

func expectedE2EArtifactTargets() ([]string, error) {
	switch runtime.GOARCH {
	case "arm64", "amd64":
		return []string{
			"ubuntu-26.04-" + runtime.GOARCH + "-qemu-k8s",
			"ubuntu-26.04-" + runtime.GOARCH + "-qemu-storage",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported host architecture for qemu e2e: %s", runtime.GOARCH)
	}
}

func localArtifactExists(repoRoot string, target string) bool {
	dir := filepath.Join(repoRoot, "kairos", "artifacts", target)
	matches, err := filepath.Glob(filepath.Join(dir, "*.raw.xz"))
	return err == nil && len(matches) > 0
}

func missingArtifactError(target string) error {
	return fmt.Errorf("missing local Kairos artifact for %s under kairos/artifacts/%s; build it with: earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=%s", target, target, target)
}

func e2eAcceptanceManifest(namespace string, pvcSize string, storageClass string) ([]byte, error) {
	docs := []map[string]any{
		{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": namespace,
			},
		},
		{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]any{
				"name":      e2ePVCName,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"accessModes":      []string{"ReadWriteOnce"},
				"storageClassName": storageClass,
				"resources": map[string]any{
					"requests": map[string]string{"storage": pvcSize},
				},
			},
		},
		{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      e2ePodName,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"restartPolicy": "Never",
				"containers": []map[string]any{{
					"name":    "io",
					"image":   "busybox:1.36",
					"command": []string{"sh", "-c", "sleep 3600"},
					"volumeMounts": []map[string]string{{
						"name":      "data",
						"mountPath": "/data",
					}},
				}},
				"volumes": []map[string]any{{
					"name": "data",
					"persistentVolumeClaim": map[string]string{
						"claimName": e2ePVCName,
					},
				}},
			},
		},
	}

	var out strings.Builder
	enc := yaml.NewEncoder(&out)
	defer enc.Close()
	for i, doc := range docs {
		if i > 0 {
			out.WriteString("---\n")
		}
		if err := enc.Encode(doc); err != nil {
			return nil, err
		}
	}
	return []byte(out.String()), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func e2eIOCheckScript(pattern string) string {
	sum := sha256.Sum256([]byte(pattern))
	return fmt.Sprintf("set -eu; printf %%s %s >/data/pattern; sync; test \"$(sha256sum /data/pattern | awk '{print $1}')\" = %s", shellQuote(pattern), shellQuote(hex.EncodeToString(sum[:])))
}

func parseSimpleQuantityBytes(value string) (int64, error) {
	value = strings.TrimSpace(value)
	units := []struct {
		suffix string
		scale  int64
	}{
		{"Gi", 1024 * 1024 * 1024},
		{"Mi", 1024 * 1024},
		{"Ki", 1024},
		{"G", 1000 * 1000 * 1000},
		{"M", 1000 * 1000},
		{"K", 1000},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			n, err := parsePositiveInt64(strings.TrimSuffix(value, unit.suffix))
			if err != nil {
				return 0, err
			}
			return n * unit.scale, nil
		}
	}
	return parsePositiveInt64(value)
}

func parsePositiveInt64(value string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse quantity %q: %w", value, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("quantity %q must be positive", value)
	}
	return n, nil
}

func writeE2EKeyPair(dir string) (privatePath string, publicPath string, publicKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", "", err
	}
	block, err := ssh.MarshalPrivateKey(priv, "k2 e2e operator")
	if err != nil {
		return "", "", "", err
	}
	privatePath = filepath.Join(dir, "operator_ed25519")
	publicPath = privatePath + ".pub"
	privateData := pem.EncodeToMemory(block)
	publicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	if err := os.WriteFile(privatePath, privateData, 0o600); err != nil {
		return "", "", "", err
	}
	if err := os.WriteFile(publicPath, []byte(publicKey+"\n"), 0o644); err != nil {
		return "", "", "", err
	}
	return privatePath, publicPath, publicKey, nil
}
