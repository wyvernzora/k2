package nodeconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeNodeFile(t *testing.T, root, target, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "clusters", target, "nodes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	cfg, found, err := Load(t.TempDir(), "v3", "k2-qm-0000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
	if len(cfg.NICs) != 0 || len(cfg.Labels) != 0 {
		t.Fatal("expected zero config")
	}
}

func TestLoadFullConfig(t *testing.T) {
	root := t.TempDir()
	writeNodeFile(t, root, "v3", "k2-qm-5cc8", `
labels = ["role=worker"]
taints = []
node_ip = "10.12.9.228"

[[nic]]
iface = "ens18"
address = "10.10.9.228/16"
gateway = "10.10.1.1"
dns = ["10.10.1.1"]

[[nic]]
iface = "ens19"
address = "172.16.9.228/16"
mtu = 9000

[[nic]]
iface = "ens18.12"
address = "10.12.9.228/16"
vlan_parent = "ens18"
vlan_id = 12
`)
	cfg, found, err := Load(root, "v3", "k2-qm-5cc8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if len(cfg.NICs) != 3 {
		t.Fatalf("expected 3 nics, got %d", len(cfg.NICs))
	}
	if cfg.NICs[1].Gateway != "" || len(cfg.NICs[1].DNS) != 0 {
		t.Fatal("fabric nic must have no gateway/dns")
	}
	if got := cfg.PrimaryAddress(); got != "10.10.9.228" {
		t.Fatalf("PrimaryAddress = %q, want 10.10.9.228", got)
	}
	if cfg.NodeIP != "10.12.9.228" {
		t.Fatalf("NodeIP = %q, want 10.12.9.228", cfg.NodeIP)
	}
	if cfg.NICs[2].VLANParent != "ens18" || cfg.NICs[2].VLANID != 12 {
		t.Fatalf("VLAN = %q/%d, want ens18/12", cfg.NICs[2].VLANParent, cfg.NICs[2].VLANID)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"unknown key":      "nodename = \"typo\"\n",
		"missing iface":    "[[nic]]\naddress = \"10.0.0.1/16\"\n",
		"non-cidr address": "[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1\"\n",
		"bad gateway":      "[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1/16\"\ngateway = \"nope\"\n",
		"bad dns":          "[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1/16\"\ndns = [\"nope\"]\n",
		"mtu out of range": "[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1/16\"\nmtu = 100\n",
		"duplicate iface":  "[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1/16\"\n[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.2/16\"\n",
		"vlan id without parent": `
[[nic]]
iface = "ens18.12"
address = "10.12.0.1/16"
vlan_id = 12
`,
		"vlan parent without id": `
[[nic]]
iface = "ens18.12"
address = "10.12.0.1/16"
vlan_parent = "ens18"
`,
		"vlan id out of range": `
[[nic]]
iface = "ens18.4095"
address = "10.12.0.1/16"
vlan_parent = "ens18"
vlan_id = 4095
`,
		"missing vlan parent": `
[[nic]]
iface = "ens18.12"
address = "10.12.0.1/16"
vlan_parent = "ens18"
vlan_id = 12
`,
		"self vlan parent": `
[[nic]]
iface = "ens18.12"
address = "10.12.0.1/16"
vlan_parent = "ens18.12"
vlan_id = 12
`,
		"stacked vlan": `
[[nic]]
iface = "ens18"
address = "10.10.0.1/16"
vlan_parent = "eth0"
vlan_id = 10

[[nic]]
iface = "ens18.12"
address = "10.12.0.1/16"
vlan_parent = "ens18"
vlan_id = 12

[[nic]]
iface = "eth0"
address = "192.0.2.1/24"
`,
		"duplicate vlan": `
[[nic]]
iface = "ens18"
address = "10.10.0.1/16"

[[nic]]
iface = "cluster0"
address = "10.12.0.1/16"
vlan_parent = "ens18"
vlan_id = 12

[[nic]]
iface = "cluster1"
address = "10.13.0.1/16"
vlan_parent = "ens18"
vlan_id = 12
`,
		"bad node ip":        "node_ip = \"nope\"\n[[nic]]\niface = \"ens18\"\naddress = \"10.0.0.1/16\"\n",
		"unassigned node ip": "node_ip = \"10.12.0.1\"\n[[nic]]\niface = \"ens18\"\naddress = \"10.10.0.1/16\"\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeNodeFile(t, root, "v3", "bad", content)
			if _, _, err := Load(root, "v3", "bad"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestPrimaryAddressEmptyWithoutNICs(t *testing.T) {
	if got := (Config{}).PrimaryAddress(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
