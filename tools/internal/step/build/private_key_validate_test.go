package build

import (
	"strings"
	"testing"
)

func TestValidateNoEmbeddedPrivateKeysRejectsSecretKeyData(t *testing.T) {
	for _, field := range []string{"data", "stringData"} {
		t.Run(field, func(t *testing.T) {
			manifest := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: hubble-tls\n" + field + ":\n  tls.key: private\n"

			err := validateNoEmbeddedPrivateKeys([]byte(manifest))
			if err == nil {
				t.Fatal("expected embedded private key to fail validation")
			}
			if !strings.Contains(err.Error(), "Secret hubble-tls embeds private key field "+field+".tls.key") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateNoEmbeddedPrivateKeysRejectsCAKeyData(t *testing.T) {
	manifest := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: cilium-ca\ndata:\n  ca.key: private\n")

	if err := validateNoEmbeddedPrivateKeys(manifest); err == nil {
		t.Fatal("expected embedded CA key to fail validation")
	}
}

func TestValidateNoEmbeddedPrivateKeysAllowsNonKeySecretData(t *testing.T) {
	manifest := []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: alertmanager\ndata:\n  alertmanager.yaml.gz: config\n  tls.crt: certificate\n")

	if err := validateNoEmbeddedPrivateKeys(manifest); err != nil {
		t.Fatalf("validateNoEmbeddedPrivateKeys: %v", err)
	}
}
