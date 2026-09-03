// Package nodeconfig loads per-node provisioning config from
// clusters/<target>/nodes/<name>.toml. The file is the durable identity
// record for a node: labels, taints, node IP, and static NIC configuration. Nodes
// without a file (test VMs, DHCP-only nodes) provision exactly as before.
package nodeconfig

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Labels []string `toml:"labels"`
	Taints []string `toml:"taints"`
	NodeIP string   `toml:"node_ip"`
	NICs   []NIC    `toml:"nic"`
}

type NIC struct {
	Iface      string   `toml:"iface"`
	Address    string   `toml:"address"` // CIDR, e.g. 10.10.9.228/16
	Gateway    string   `toml:"gateway,omitempty"`
	DNS        []string `toml:"dns,omitempty"`
	MTU        int      `toml:"mtu,omitempty"`
	VLANParent string   `toml:"vlan_parent,omitempty"`
	VLANID     int      `toml:"vlan_id,omitempty"`
	Routes     []Route  `toml:"route,omitempty"`
}

type Route struct {
	Destination     string `toml:"destination"`
	Gateway         string `toml:"gateway,omitempty"`
	PreferredSource string `toml:"preferred_source,omitempty"`
}

// Path returns where the node file for the given node lives.
func Path(repoRoot, target, nodeName string) string {
	return filepath.Join(repoRoot, "clusters", target, "nodes", nodeName+".toml")
}

// Load reads and validates the node file. The second return is false when no
// file exists for the node, which is not an error.
func Load(repoRoot, target, nodeName string) (Config, bool, error) {
	path := Path(repoRoot, target, nodeName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read node config %s: %w", path, err)
	}

	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, false, fmt.Errorf("parse node config %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return Config{}, false, fmt.Errorf("node config %s: unknown keys: %s", path, strings.Join(keys, ", "))
	}
	if err := cfg.validate(path); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

// PrimaryAddress returns the IP (without prefix) of the first configured NIC,
// or "" when the node has no static NICs. After provisioning reboots a node,
// this is the address it comes back on; the original DHCP bootstrap address
// is not guaranteed to exist anymore.
func (c Config) PrimaryAddress() string {
	if len(c.NICs) == 0 {
		return ""
	}
	ip, _, err := net.ParseCIDR(c.NICs[0].Address)
	if err != nil {
		return ""
	}
	return ip.String()
}

func (c Config) validate(path string) error {
	seen := map[string]bool{}
	for i, nic := range c.NICs {
		at := fmt.Sprintf("%s: nic[%d]", path, i)
		if nic.Iface == "" {
			return fmt.Errorf("%s: missing iface", at)
		}
		if seen[nic.Iface] {
			return fmt.Errorf("%s: duplicate iface %q", at, nic.Iface)
		}
		seen[nic.Iface] = true
		_, _, err := net.ParseCIDR(nic.Address)
		if err != nil {
			return fmt.Errorf("%s: address must be CIDR (e.g. 10.10.9.228/16): %w", at, err)
		}
		if nic.Gateway != "" && net.ParseIP(nic.Gateway) == nil {
			return fmt.Errorf("%s: invalid gateway %q", at, nic.Gateway)
		}
		for _, d := range nic.DNS {
			if net.ParseIP(d) == nil {
				return fmt.Errorf("%s: invalid dns %q", at, d)
			}
		}
		if nic.MTU != 0 && (nic.MTU < 576 || nic.MTU > 9216) {
			return fmt.Errorf("%s: mtu %d out of range [576, 9216]", at, nic.MTU)
		}
		if err := validateRoutes(at, nic.Routes); err != nil {
			return err
		}
	}
	if err := validateVLANs(path, c.NICs); err != nil {
		return err
	}
	return validateNodeIP(path, c.NodeIP, c.NICs)
}

func validateVLANs(path string, nics []NIC) error {
	byIface := make(map[string]NIC, len(nics))
	for _, nic := range nics {
		byIface[nic.Iface] = nic
	}
	type vlanKey struct {
		parent string
		id     int
	}
	seen := make(map[vlanKey]bool)
	for i, nic := range nics {
		at := fmt.Sprintf("%s: nic[%d]", path, i)
		hasParent := nic.VLANParent != ""
		hasID := nic.VLANID != 0
		if hasParent != hasID {
			return fmt.Errorf("%s: vlan_parent and vlan_id must be set together", at)
		}
		if !hasParent {
			continue
		}
		if nic.VLANID < 1 || nic.VLANID > 4094 {
			return fmt.Errorf("%s: vlan_id %d out of range [1, 4094]", at, nic.VLANID)
		}
		if nic.VLANParent == nic.Iface {
			return fmt.Errorf("%s: vlan_parent must differ from iface", at)
		}
		parent, ok := byIface[nic.VLANParent]
		if !ok {
			return fmt.Errorf("%s: vlan_parent %q is not a configured nic", at, nic.VLANParent)
		}
		if parent.VLANParent != "" {
			return fmt.Errorf("%s: vlan_parent %q must be a physical nic", at, nic.VLANParent)
		}
		key := vlanKey{parent: nic.VLANParent, id: nic.VLANID}
		if seen[key] {
			return fmt.Errorf("%s: duplicate vlan_id %d on parent %q", at, nic.VLANID, nic.VLANParent)
		}
		seen[key] = true
	}
	return nil
}

func validateRoutes(nicPath string, routes []Route) error {
	for i, route := range routes {
		at := fmt.Sprintf("%s: route[%d]", nicPath, i)
		if _, _, err := net.ParseCIDR(route.Destination); err != nil {
			return fmt.Errorf("%s: destination must be CIDR: %w", at, err)
		}
		if route.Gateway != "" && net.ParseIP(route.Gateway) == nil {
			return fmt.Errorf("%s: invalid gateway %q", at, route.Gateway)
		}
		if route.PreferredSource != "" && net.ParseIP(route.PreferredSource) == nil {
			return fmt.Errorf("%s: invalid preferred_source %q", at, route.PreferredSource)
		}
	}
	return nil
}

func validateNodeIP(path, raw string, nics []NIC) error {
	if raw == "" {
		return nil
	}
	nodeIP := net.ParseIP(raw)
	if nodeIP == nil {
		return fmt.Errorf("%s: invalid node_ip %q", path, raw)
	}
	for _, nic := range nics {
		ip, _, _ := net.ParseCIDR(nic.Address)
		if nodeIP.Equal(ip) {
			return nil
		}
	}
	return fmt.Errorf("%s: node_ip %q is not assigned to a configured nic", path, raw)
}
