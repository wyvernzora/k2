package render

import (
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
	"gopkg.in/yaml.v3"
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
	for _, forbidden := range []string{".netdev", "VLAN="} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("physical-only config must not contain %q:\n%s", forbidden, got)
		}
	}
}

func TestNetworkActivationCloudConfigVLAN(t *testing.T) {
	got := NetworkActivationCloudConfig([]nodeconfig.NIC{
		{
			Iface: "end0", Address: "10.10.9.10/16", Gateway: "10.10.1.1", DNS: []string{"10.10.10.10"},
		},
		{
			Iface: "end0.12", Address: "10.12.9.10/16", VLANParent: "end0", VLANID: 12,
		},
	})
	var config struct {
		Stages activationStages `yaml:"stages"`
	}
	if err := yaml.Unmarshal(got, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Stages.Initramfs) != 1 {
		t.Fatalf("initramfs stages = %d, want 1", len(config.Stages.Initramfs))
	}
	files := make(map[string]string, len(config.Stages.Initramfs[0].Files))
	for _, file := range config.Stages.Initramfs[0].Files {
		files[file.Path] = file.Content
	}
	want := map[string]string{
		"/etc/ssh/sshd_config.d/20-k2-listen-address.conf": "ListenAddress 10.10.9.10\n",
		"/etc/systemd/network/10-k2-end0.12.netdev":        "[NetDev]\nName=end0.12\nKind=vlan\n\n[VLAN]\nId=12\n",
		"/etc/systemd/network/10-k2-end0.12.network":       "[Match]\nName=end0.12\n\n[Network]\nAddress=10.12.9.10/16\n",
		"/etc/systemd/network/10-k2-end0.network": `[Match]
Name=end0

[Network]
Address=10.10.9.10/16
VLAN=end0.12
Gateway=10.10.1.1
DNS=10.10.10.10
`,
	}
	if len(files) != len(want) {
		t.Fatalf("rendered files = %d, want %d: %#v", len(files), len(want), files)
	}
	for path, content := range want {
		if files[path] != content {
			t.Errorf("%s content:\n%s\nwant:\n%s", path, files[path], content)
		}
	}
}
