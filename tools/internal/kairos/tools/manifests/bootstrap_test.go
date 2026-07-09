package manifests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
)

func TestBootstrapAssemblesMinimumPayload(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deploy", "cilium", "crds.k8s.yaml"), "kind: CustomResourceDefinition\nmetadata:\n  name: cilium\n")
	write(t, filepath.Join(root, "deploy", "cilium", "app.k8s.yaml"), "kind: DaemonSet\nmetadata:\n  name: cilium\n")
	write(t, filepath.Join(root, "deploy", "argocd", "crds.k8s.yaml"), "kind: CustomResourceDefinition\nmetadata:\n  name: argocd\n")
	write(t, filepath.Join(root, "deploy", "argocd", "app.k8s.yaml"), "kind: Deployment\nmetadata:\n  name: argocd\n")
	write(t, filepath.Join(root, "deploy", "kube-vip", "app.k8s.yaml"), "kind: DaemonSet\nmetadata:\n  name: kube-vip\n")
	extraPath := filepath.Join(root, "extra", "onepassword-token.yaml")
	write(t, extraPath, `apiVersion: v1
kind: Secret
metadata:
  name: onepassword-token
  namespace: external-secrets
type: Opaque
stringData:
  token: op-token
`)

	cfg := testClusterConfig()
	gotBytes, err := Bootstrap(root, cfg, BootstrapOptions{
		ExtraManifestPatterns: []string{filepath.Join(root, "extra", "*.yaml")},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	for _, want := range []string{
		"name: cilium",
		"name: argocd",
		"name: kube-vip",
		"name: external-secrets",
		"name: onepassword-token",
		"stringData:",
		"token: op-token",
		"name: k2-bootstrap-manifest-cleanup",
		"rm -f /host-manifests/k2-bootstrap.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("payload missing %q:\n%s", want, got)
		}
	}
	// The bootstrap bundle pre-creates the external-secrets namespace itself,
	// so the auto-prepend pass must not add a second Namespace/external-secrets.
	if strings.Count(got, "name: external-secrets\n") != 1 {
		t.Fatalf("expected exactly one Namespace/external-secrets, got:\n%s", got)
	}
	if strings.Contains(got, "name: root") {
		t.Fatalf("bootstrap static bundle should not include root Argo CD app manifest:\n%s", got)
	}
}

func TestRootArgoAppRendersAppOfApps(t *testing.T) {
	cfg := testClusterConfig()
	got, err := RootArgoApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(got)
	for _, want := range []string{
		"kind: Application",
		"name: k2",
		"namespace: argocd",
		"repoURL: https://github.com/wyvernzora/k2.git",
		"targetRevision: deploy",
		"path: .",
		"include: app.k8s.yaml",
		"ServerSideApply=true",
	} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("root app manifest missing %q:\n%s", want, manifest)
		}
	}
	// CONTRACT, not style: provisioning applies this Application to TEST
	// clusters too, relying on it never auto-syncing — without automated
	// sync it sits OutOfSync and deploys nothing until a human syncs it.
	// If production ever needs automated sync, it must be opt-in per
	// cluster and stay off for test-VM bundles.
	for _, unwanted := range []string{"automated:", "prune:", "selfHeal:"} {
		if strings.Contains(manifest, unwanted) {
			t.Fatalf("root app manifest should not include %q:\n%s", unwanted, manifest)
		}
	}
}

func TestBootstrapPatchesCiliumAPIHost(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deploy", "cilium", "crds.k8s.yaml"), "kind: CustomResourceDefinition\nmetadata:\n  name: cilium\n")
	write(t, filepath.Join(root, "deploy", "cilium", "app.k8s.yaml"), `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: cilium
spec:
  template:
    spec:
      containers:
        - name: cilium-agent
          env:
            - name: KUBERNETES_SERVICE_HOST
              value: 10.10.9.1
            - name: KUBERNETES_SERVICE_PORT
              value: "6443"
`)
	write(t, filepath.Join(root, "deploy", "argocd", "crds.k8s.yaml"), "kind: CustomResourceDefinition\nmetadata:\n  name: argocd\n")
	write(t, filepath.Join(root, "deploy", "argocd", "app.k8s.yaml"), "kind: Deployment\nmetadata:\n  name: argocd\n")
	write(t, filepath.Join(root, "deploy", "kube-vip", "app.k8s.yaml"), "kind: DaemonSet\nmetadata:\n  name: kube-vip\n")

	cfg := testClusterConfig()
	gotBytes, err := Bootstrap(root, cfg, BootstrapOptions{CiliumAPIHost: "10.0.2.15"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "value: 10.0.2.15") {
		t.Fatalf("bootstrap Cilium API host was not patched:\n%s", got)
	}
	if strings.Contains(got, "value: 10.10.9.1") {
		t.Fatalf("bootstrap Cilium API host still contains VIP:\n%s", got)
	}
}

func testClusterConfig() clusterconfig.Config {
	return clusterconfig.Config{
		ID: "v3",
		Argo: clusterconfig.Argo{
			Namespace:  "argocd",
			Project:    "default",
			RepoURL:    "https://github.com/wyvernzora/k2.git",
			RepoBranch: "deploy",
		},
	}
}

func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
