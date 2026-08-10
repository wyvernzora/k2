package upgrade

import (
	"fmt"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/render"
	stepupgrade "github.com/wyvernzora/k2/tools/internal/step/upgrade"
)

func readUpgradeMetadata(client *remote.Client) (stepupgrade.NodeImageMetadata, error) {
	data, err := client.ReadFile("/usr/share/k2/image-build/metadata.yaml")
	if err != nil {
		return stepupgrade.NodeImageMetadata{}, fmt.Errorf("read remote image metadata: %w", err)
	}
	m, err := render.DecodeImageMetadata(data)
	if err != nil {
		return stepupgrade.NodeImageMetadata{}, err
	}
	if m.Target == "" || m.Arch == "" || m.Hardware == "" {
		return stepupgrade.NodeImageMetadata{}, fmt.Errorf("remote image metadata is incomplete; target, arch, and hardware are required")
	}
	return stepupgrade.NodeImageMetadata{
		Target: m.Target, Flavor: m.Flavor, FlavorRelease: m.FlavorRelease, Variant: m.Variant,
		Role: m.Role, Arch: m.Arch, Hardware: m.Hardware, KubernetesDistro: m.KubernetesDistro,
		KubernetesVersion: m.KubernetesVersion, KairosVersion: m.KairosVersion, SourceCommit: m.SourceCommit,
		DiskStateSizeMiB: m.DiskStateSizeMiB, UpgradeSizeAllowanceMiB: m.UpgradeSizeAllowanceMiB,
		RootFSSizeMiB: m.RootFSSizeMiB, AppliedDigest: readAppliedDigest(client),
	}, nil
}

// readAppliedDigest returns the digest k2-tools recorded at the node's last
// upgrade, or "" when the file is absent — the expected state for a node
// installed from a raw artifact, which falls back to source-commit identity.
func readAppliedDigest(client *remote.Client) string {
	data, err := client.ReadFile(stepupgrade.AppliedDigestPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
