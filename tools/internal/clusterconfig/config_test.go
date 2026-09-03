package clusterconfig

import (
	"strings"
	"testing"
)

func TestAPIServerURLUsesPrimaryVIP(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Kubernetes: Kubernetes{
			API: KubernetesAPI{
				Primary: "kube-vip-cluster",
				VIPs: []APIVIP{
					{Name: "kube-vip", Address: "10.10.9.1", Interface: "end0"},
					{Name: "kube-vip-cluster", Address: "10.12.9.1", Interface: "end0.12"},
				},
			},
		},
	}

	if got, want := cfg.APIServerURL(), "https://10.12.9.1:6443"; got != want {
		t.Fatalf("APIServerURL() = %q, want %q", got, want)
	}
}

func TestKubernetesAPIAddressesPreserveVIPOrder(t *testing.T) {
	t.Parallel()

	api := KubernetesAPI{
		VIPs: []APIVIP{
			{Name: "kube-vip", Address: "10.10.9.1"},
			{Name: "kube-vip-cluster", Address: "10.12.9.1"},
		},
	}

	if got, want := strings.Join(api.Addresses(), ","), "10.10.9.1,10.12.9.1"; got != want {
		t.Fatalf("Addresses() = %q, want %q", got, want)
	}
}

func TestKubernetesAPIValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		api       KubernetesAPI
		wantError string
	}{
		{
			name: "valid multiple VIPs",
			api: KubernetesAPI{
				Primary: "kube-vip",
				VIPs: []APIVIP{
					{Name: "kube-vip", Address: "10.10.9.1", Interface: "end0"},
					{Name: "kube-vip-cluster", Address: "10.12.9.1", Interface: "end0.12"},
				},
			},
		},
		{
			name:      "missing VIPs",
			api:       KubernetesAPI{Primary: "kube-vip"},
			wantError: "api.vips: must not be empty",
		},
		{
			name: "unknown primary",
			api: KubernetesAPI{
				Primary: "missing",
				VIPs:    []APIVIP{{Name: "kube-vip", Address: "10.10.9.1"}},
			},
			wantError: `api.primary: no VIP named "missing"`,
		},
		{
			name: "duplicate name",
			api: KubernetesAPI{
				Primary: "kube-vip",
				VIPs: []APIVIP{
					{Name: "kube-vip", Address: "10.10.9.1"},
					{Name: "kube-vip", Address: "10.12.9.1"},
				},
			},
			wantError: `api.vips[1].name: duplicate name "kube-vip"`,
		},
		{
			name: "duplicate address",
			api: KubernetesAPI{
				Primary: "kube-vip",
				VIPs: []APIVIP{
					{Name: "kube-vip", Address: "10.10.9.1"},
					{Name: "kube-vip-cluster", Address: "10.10.9.1"},
				},
			},
			wantError: `api.vips[1].address: duplicate address "10.10.9.1"`,
		},
		{
			name: "invalid name",
			api: KubernetesAPI{
				Primary: "Kube VIP",
				VIPs:    []APIVIP{{Name: "Kube VIP", Address: "10.10.9.1"}},
			},
			wantError: `api.vips[0].name: "Kube VIP" is not a DNS label`,
		},
		{
			name: "blank optional interface",
			api: KubernetesAPI{
				Primary: "kube-vip",
				VIPs:    []APIVIP{{Name: "kube-vip", Address: "10.10.9.1", Interface: " "}},
			},
			wantError: "api.vips[0].interface: must not be blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.api.validate("api")
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("validate() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}
