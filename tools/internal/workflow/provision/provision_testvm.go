package provision

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/wyvernzora/k2/tools/internal/clusterconfig"
	testvm "github.com/wyvernzora/k2/tools/internal/step/vm"
)

func applyProvisionTestVM(repoRoot string, clusterTarget string, clusterName *string, nodeName *string, host *string, sshPort *int, vmID string) (testVMProvisionTarget, error) {
	if vmID == "" {
		if *host == "" {
			return testVMProvisionTarget{}, fmt.Errorf("missing SSH host; pass --host or --test-vm")
		}
		return testVMProvisionTarget{}, nil
	}

	target, err := testvm.ResolveProvisionTarget(repoRoot, vmID)
	if err != nil {
		return testVMProvisionTarget{}, err
	}
	if *clusterName == "" {
		*clusterName = clusterTarget + "-vmtest"
	}
	if *nodeName == "" {
		*nodeName = vmID
	}
	*host = target.Host
	*sshPort = target.Port

	out := testVMProvisionTarget{Enabled: true, GuestIP: target.GuestIPv4.Address}
	if target.GuestIPv4.Address != "" {
		vip, err := testKubeVIP(target.GuestIPv4.Address, target.GuestIPv4.Prefix)
		if err != nil {
			return testVMProvisionTarget{}, err
		}
		out.KubeVIP = vip
	}
	return out, nil
}

func applyTestKubeVIP(cfg *clusterconfig.Config, vip string) {
	cfg.Kubernetes.API = vip
}

func testKubeVIP(nodeIP string, prefix int) (string, error) {
	parsed := net.ParseIP(nodeIP).To4()
	if parsed == nil {
		return "", fmt.Errorf("test VM node address %q is not IPv4", nodeIP)
	}
	if prefix <= 0 || prefix >= 31 {
		return "", fmt.Errorf("test VM node address %s has unsupported prefix %d", nodeIP, prefix)
	}

	ip := binary.BigEndian.Uint32(parsed)
	mask := uint32(0xffffffff) << (32 - prefix)
	network := ip & mask
	broadcast := network | ^mask
	candidate := broadcast - 1
	if candidate == ip {
		candidate--
	}
	if candidate <= network {
		return "", fmt.Errorf("could not choose test VM kube-vip address in %s/%d", nodeIP, prefix)
	}

	var out [4]byte
	binary.BigEndian.PutUint32(out[:], candidate)
	return net.IP(out[:]).String(), nil
}
