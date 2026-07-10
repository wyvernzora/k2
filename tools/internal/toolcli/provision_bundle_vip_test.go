package toolcli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/render"
)

// The deploy-repo manifests carry the production VIP as a literal; test-VM
// bundles must substitute it at render time. The old approach (patching the
// live DaemonSet post-boot) raced k3s's deploy controller, which re-applies
// the manifest on its own watcher events and reverted the patch.
func TestBuildBundleSubstitutesTestKubeVIPInManifests(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	// deploy/ is generated output (gitignored), so this test can only run
	// where the manifests have been rendered — locally, not CI.
	if _, err := os.Stat(filepath.Join(root, "deploy")); os.IsNotExist(err) {
		t.Skip("deploy/ manifests not rendered; run the k8s manifest build first")
	}
	flags := commonBootstrapFlags{
		ClusterTarget:    "v3",
		OperatorKey:      []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPZ2bnq8V3zhXTIvYRnvbUFYY1B5Z0T1p1r2WYyRr2W1 test"},
		NodeName:         "e2e-test",
		BootstrapAPIHost: "192.168.2.10",
		testKubeVIP:      "192.168.2.254",
	}
	got, err := buildBundle(root, flags, render.ImageMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got.Manifests, []byte("192.168.2.254")) {
		t.Fatal("bootstrap manifests do not contain the test kube-vip address")
	}
	if bytes.Contains(got.Manifests, []byte("10.10.9.1")) {
		t.Fatal("bootstrap manifests still contain the production VIP")
	}
}
