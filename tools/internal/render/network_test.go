package render

import (
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
)

func TestNetworkActivationCloudConfig(t *testing.T) {
	got := string(NetworkActivationCloudConfig([]nodeconfig.NIC{
		{
			Iface: "ens18", Address: "10.10.9.228/16", Gateway: "10.10.1.1", DNS: []string{"10.10.1.1"},
			Routes: []nodeconfig.Route{{Destination: "10.42.4.0/24", Gateway: "10.10.9.187", PreferredSource: "10.12.9.228"}},
		},
		{Iface: "ens19", Address: "172.16.9.228/16", MTU: 9000},
		{
			Iface: "ens20", Address: "10.12.9.228/16",
			Routes: []nodeconfig.Route{{Destination: "10.10.9.187/32", Gateway: "10.12.1.1", PreferredSource: "10.12.9.228"}},
		},
	}))
	if !strings.HasPrefix(got, "#cloud-config\n") {
		t.Fatalf("missing #cloud-config header:\n%s", got)
	}
	for _, want := range []string{
		"/etc/systemd/network/10-k2-ens18.network",
		"/etc/systemd/network/10-k2-ens19.network",
		"/etc/systemd/network/10-k2-ens20.network",
		"/etc/ssh/sshd_config.d/20-k2-listen-address.conf",
		"Name=ens18",
		"Address=10.10.9.228/16",
		"Gateway=10.10.1.1",
		"DNS=10.10.1.1",
		"Destination=10.42.4.0/24",
		"Gateway=10.10.9.187",
		"PreferredSource=10.12.9.228",
		"Name=ens19",
		"Address=172.16.9.228/16",
		"MTUBytes=9000",
		"Name=ens20",
		"Address=10.12.9.228/16",
		"Destination=10.10.9.187/32",
		"Gateway=10.12.1.1",
		"ListenAddress 10.10.9.228",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// The fabric NIC must not inherit gateway/DNS from the LAN NIC.
	idx := strings.Index(got, "Name=ens19")
	if idx < 0 {
		t.Fatal("fabric nic unit missing")
	}
	fabric := got[idx:]
	if end := strings.Index(fabric, "/etc/systemd/network/10-k2-ens20.network"); end >= 0 {
		fabric = fabric[:end]
	}
	for _, forbidden := range []string{"Gateway=", "DNS="} {
		if strings.Contains(fabric, forbidden) {
			t.Errorf("fabric nic unit must not contain %q:\n%s", forbidden, fabric)
		}
	}
	for _, forbidden := range []string{"ListenAddress 172.16.9.228", "ListenAddress 10.12.9.228"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sshd must not listen on a non-management fabric (%s):\n%s", forbidden, got)
		}
	}
}
