package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
)

// NetworkActivationCloudConfig renders the /oem stage that pins static NIC
// configuration. It writes one 10-k2-<iface>.network file per NIC; systemd-
// networkd applies the first matching file in lexical order, so these win
// over the image-baked 20-dhcp.network catch-all without touching it. /etc
// is ephemeral on Kairos, which is why the files are (re)written from /oem
// on every boot rather than installed once.
func NetworkActivationCloudConfig(nics []nodeconfig.NIC) []byte {
	type config struct {
		Name   string           `yaml:"name"`
		Stages activationStages `yaml:"stages"`
	}
	files := make([]activationFile, 0, len(nics))
	for _, nic := range nics {
		files = append(files, activationFile{
			Path:        "/etc/systemd/network/10-k2-" + nic.Iface + ".network",
			Content:     networkdUnit(nic),
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
					Name:  "Pin static NIC configuration",
					Files: files,
				},
			},
		},
	}
	return mustCloudConfig(out)
}

func networkdUnit(nic nodeconfig.NIC) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Match]\nName=%s\n", nic.Iface)
	if nic.MTU != 0 {
		fmt.Fprintf(&b, "\n[Link]\nMTUBytes=%d\n", nic.MTU)
	}
	fmt.Fprintf(&b, "\n[Network]\nAddress=%s\n", nic.Address)
	if nic.Gateway != "" {
		fmt.Fprintf(&b, "Gateway=%s\n", nic.Gateway)
	}
	for _, dns := range nic.DNS {
		fmt.Fprintf(&b, "DNS=%s\n", dns)
	}
	return b.String()
}
