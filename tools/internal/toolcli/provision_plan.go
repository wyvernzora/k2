package toolcli

import (
	"fmt"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/manifests"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

var extraManifestHeaders = []string{"APIVERSION/KIND", "NAMESPACE", "NAME"}

func clusterNameOrFallback(name, target string) string {
	if name == "" {
		return target
	}
	return name
}

func nodeLabelsForRole(role nodeRole, labels []string) ([]string, error) {
	if err := rejectLonghornNodeLabels(string(role), labels); err != nil {
		return nil, err
	}
	return append([]string{}, labels...), nil
}

func rejectLonghornNodeLabels(role string, labels []string) error {
	for _, label := range labels {
		key := strings.TrimSpace(label)
		if before, _, found := strings.Cut(key, "="); found {
			key = strings.TrimSpace(before)
		}
		if strings.HasPrefix(key, longhornNodeLabelPrefix) {
			return fmt.Errorf("%s provisioning does not allow user-supplied Longhorn node label %q; k2-tools manages Longhorn replica-storage labels after worker join", role, key)
		}
	}
	return nil
}

func bootstrapPlanFields(c *bootstrapCmd, testTarget testVMProvisionTarget) []ui.KV {
	pairs := []ui.KV{
		{Key: "Cluster target", Value: c.ClusterTarget},
		{Key: "Cluster name", Value: clusterNameOrFallback(c.ClusterName, c.ClusterTarget)},
		{Key: "Node name", Value: c.NodeName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", c.SSHUser, c.Host, c.SSHPort)},
		{Key: "Operator keys", Value: keysSummary(c.OperatorKey, c.OperatorFiles)},
		{Key: "Labels", Value: joinOrNone(c.Label)},
		{Key: "Taints", Value: joinOrNone(c.Taint)},
		{Key: "Bootstrap API host", Value: hostOrAutoDetect(c.BootstrapAPIHost)},
		{Key: "Reboot after install", Value: yesNo(!c.NoReboot)},
	}
	if c.TestVM != "" {
		pairs = append(pairs,
			ui.KV{Key: "Test VM", Value: c.TestVM},
		)
		if testTarget.KubeVIP != "" {
			pairs = append(pairs, ui.KV{Key: "Test kube-VIP", Value: testTarget.KubeVIP})
		}
	}
	return pairs
}

func joinPlanFields(role nodeRole, flags commonJoinFlags, remoteFlags commonRemoteFlags) []ui.KV {
	pairs := []ui.KV{
		{Key: "Role", Value: string(role)},
		{Key: "Cluster target", Value: flags.ClusterTarget},
		{Key: "Cluster name", Value: clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget)},
		{Key: "Node name", Value: flags.NodeName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", remoteFlags.SSHUser, remoteFlags.Host, remoteFlags.SSHPort)},
		{Key: "Operator keys", Value: keysSummary(flags.OperatorKey, flags.OperatorFiles)},
		{Key: "Labels", Value: joinOrNone(flags.Label)},
		{Key: "Taints", Value: joinOrNone(flags.Taint)},
		{Key: "Server URL", Value: hostOrFromCluster(flags.ServerURL)},
		{Key: "Reboot after install", Value: yesNo(!remoteFlags.NoReboot)},
	}
	if remoteFlags.TestVM != "" {
		pairs = append(pairs, ui.KV{Key: "Test VM", Value: remoteFlags.TestVM})
	}
	return pairs
}

func extraManifestRows(objs []manifests.ExtraManifestObject) [][]string {
	rows := make([][]string, len(objs))
	for i, o := range objs {
		ns := o.Namespace
		if ns == "" {
			ns = "(cluster-scoped)"
		}
		rows[i] = []string{o.APIVersion + "/" + o.Kind, ns, o.Name}
	}
	return rows
}

func keysSummary(literal, files []string) string {
	switch {
	case len(literal) == 0 && len(files) == 0:
		return "(none — provisioner will fail without keys)"
	case len(literal) == 0:
		return fmt.Sprintf("%d file path(s)", len(files))
	case len(files) == 0:
		return fmt.Sprintf("%d literal", len(literal))
	default:
		return fmt.Sprintf("%d literal, %d file path(s)", len(literal), len(files))
	}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func hostOrAutoDetect(host string) string {
	if host == "" {
		return "(auto-detect from node primary IPv4)"
	}
	return host
}

func hostOrFromCluster(url string) string {
	if url == "" {
		return "(default — ~/.kube/k2/<cluster>/server-url, then cluster YAML VIP)"
	}
	return url
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
