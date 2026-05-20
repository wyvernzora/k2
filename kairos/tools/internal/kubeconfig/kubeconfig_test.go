package kubeconfig

import (
	"strings"
	"testing"
)

func TestRewriteServer(t *testing.T) {
	in := []byte(`
apiVersion: v1
kind: Config
clusters:
  - name: default
    cluster:
      certificate-authority-data: abc
      server: https://127.0.0.1:6443
contexts:
  - name: default
    context:
      cluster: default
      user: default
current-context: default
users:
  - name: default
    user:
      client-certificate-data: def
      client-key-data: ghi
`)

	out, err := RewriteServer(in, "https://k8s-api.wyvernzora.io:6443")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "server: https://k8s-api.wyvernzora.io:6443") {
		t.Fatalf("rewritten kubeconfig missing server:\n%s", got)
	}
	if strings.Contains(got, "https://127.0.0.1:6443") {
		t.Fatalf("rewritten kubeconfig still contains old server:\n%s", got)
	}
}

func TestRewriteServerRejectsMissingClusters(t *testing.T) {
	_, err := RewriteServer([]byte("apiVersion: v1\nkind: Config\n"), "https://example.com:6443")
	if err == nil || !strings.Contains(err.Error(), "no clusters") {
		t.Fatalf("error = %v", err)
	}
}
