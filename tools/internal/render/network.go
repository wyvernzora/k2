package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
)

// NetworkActivationCloudConfig renders the /oem stage that pins static NIC
// configuration and binds sshd to the first, management NIC. It writes one
// 10-k2-<iface>.network file per NIC and a .netdev file for each VLAN child;
// systemd-networkd applies the first matching .network file in lexical order,
// so these win over the image-baked 20-dhcp.network catch-all without touching
// it. /etc is ephemeral on Kairos, which is why the files are (re)written from
// /oem on every boot rather than installed once.
func NetworkActivationCloudConfig(nics []nodeconfig.NIC) []byte {
	type config struct {
		Name   string           `yaml:"name"`
		Stages activationStages `yaml:"stages"`
	}
	vlanChildren := make(map[string][]string)
	for _, nic := range nics {
		if nic.VLANParent != "" {
			vlanChildren[nic.VLANParent] = append(vlanChildren[nic.VLANParent], nic.Iface)
		}
	}
	for parent := range vlanChildren {
		sort.Strings(vlanChildren[parent])
	}

	files := make([]activationFile, 0, len(nics)*2+1)
	for _, nic := range nics {
		if nic.VLANParent != "" {
			files = append(files, activationFile{
				Path:        "/etc/systemd/network/10-k2-" + nic.Iface + ".netdev",
				Content:     vlanNetDevUnit(nic),
				Permissions: 0o644,
				Owner:       0,
				Group:       0,
			})
		}
		files = append(files, activationFile{
			Path:        "/etc/systemd/network/10-k2-" + nic.Iface + ".network",
			Content:     networkdUnit(nic, vlanChildren[nic.Iface]),
			Permissions: 0o644,
			Owner:       0,
			Group:       0,
		})
	}
	if len(nics) > 0 {
		managementIP, _, _ := strings.Cut(nics[0].Address, "/")
		files = append(files, activationFile{
			Path:        "/etc/ssh/sshd_config.d/20-k2-listen-address.conf",
			Content:     fmt.Sprintf("ListenAddress %s\n", managementIP),
			Permissions: 0o644,
			Owner:       0,
			Group:       0,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	out := config{
		Name: "K2 static network",
		Stages: activationStages{
			Initramfs: []activationStage{
				{
					Name:  "Pin static NIC and SSH listener configuration",
					Files: files,
				},
			},
		},
	}
	return mustCloudConfig(out)
}

func networkdUnit(nic nodeconfig.NIC, vlanChildren []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Match]\nName=%s\n", nic.Iface)
	if nic.MTU != 0 {
		fmt.Fprintf(&b, "\n[Link]\nMTUBytes=%d\n", nic.MTU)
	}
	fmt.Fprintf(&b, "\n[Network]\nAddress=%s\n", nic.Address)
	for _, child := range vlanChildren {
		fmt.Fprintf(&b, "VLAN=%s\n", child)
	}
	if nic.Gateway != "" {
		fmt.Fprintf(&b, "Gateway=%s\n", nic.Gateway)
	}
	for _, dns := range nic.DNS {
		fmt.Fprintf(&b, "DNS=%s\n", dns)
	}
	for _, route := range nic.Routes {
		fmt.Fprintf(&b, "\n[Route]\nDestination=%s\n", route.Destination)
		if route.Gateway != "" {
			fmt.Fprintf(&b, "Gateway=%s\n", route.Gateway)
		}
		if route.PreferredSource != "" {
			fmt.Fprintf(&b, "PreferredSource=%s\n", route.PreferredSource)
		}
	}
	return b.String()
}

func vlanNetDevUnit(nic nodeconfig.NIC) string {
	return fmt.Sprintf("[NetDev]\nName=%s\nKind=vlan\n\n[VLAN]\nId=%d\n", nic.Iface, nic.VLANID)
}
