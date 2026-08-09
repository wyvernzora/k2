package render

import (
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
)

func TestNetworkActivationCloudConfig(t *testing.T) {
	got := string(NetworkActivationCloudConfig([]nodeconfig.NIC{
		{Iface: "ens18", Address: "10.10.9.228/16", Gateway: "10.10.1.1", DNS: []string{"10.10.1.1"}},
		{Iface: "ens19", Address: "172.16.9.228/16", MTU: 9000},
	}))
	if !strings.HasPrefix(got, "#cloud-config\n") {
		t.Fatalf("missing #cloud-config header:\n%s", got)
	}
	for _, want := range []string{
		"/etc/systemd/network/10-k2-ens18.network",
		"/etc/systemd/network/10-k2-ens19.network",
		"Name=ens18",
		"Address=10.10.9.228/16",
		"Gateway=10.10.1.1",
		"DNS=10.10.1.1",
		"Name=ens19",
		"Address=172.16.9.228/16",
		"MTUBytes=9000",
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
	for _, forbidden := range []string{"Gateway=", "DNS="} {
		if strings.Contains(fabric, forbidden) {
			t.Errorf("fabric nic unit must not contain %q:\n%s", forbidden, fabric)
		}
	}
}
