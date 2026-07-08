package render

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/clusterconfig"
	"gopkg.in/yaml.v3"
)

const (
	LabelHardware    = "k2.wyvernzora.io/hardware"
	LabelImageTarget = "k2.wyvernzora.io/image-target"
	LabelArch        = "k2.wyvernzora.io/arch"
)

type BootstrapInput struct {
	Cluster       clusterconfig.Config
	NodeName      string
	Labels        []string
	Taints        []string
	ImageMetadata ImageMetadata
}

type JoinInput struct {
	NodeName      string
	ServerURL     string
	Token         string
	Labels        []string
	Taints        []string
	ImageMetadata ImageMetadata
	ControlPlane  bool
}

type ImageMetadata struct {
	Target   string `yaml:"target"`
	Arch     string `yaml:"arch"`
	Hardware string `yaml:"hardware"`
	Role     string `yaml:"role"`
}

type activationStages struct {
	Initramfs []activationStage `yaml:"initramfs,omitempty"`
}

type activationStage struct {
	Name     string                    `yaml:"name"`
	Hostname string                    `yaml:"hostname,omitempty"`
	Commands []string                  `yaml:"commands,omitempty"`
	Users    map[string]activationUser `yaml:"users,omitempty"`
}

type activationUser struct {
	Groups            []string `yaml:"groups,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
}

func ClusterConfig(c clusterconfig.Config) ([]byte, error) {
	type config struct {
		ClusterCIDR       string   `yaml:"cluster-cidr"`
		ServiceCIDR       string   `yaml:"service-cidr"`
		ClusterDNS        string   `yaml:"cluster-dns"`
		ClusterDomain     string   `yaml:"cluster-domain"`
		TLSSAN            []string `yaml:"tls-san,omitempty"`
		KubeAPIServerArgs []string `yaml:"kube-apiserver-arg,omitempty"`
	}
	kubeAPIServerArgs := []string{}
	if c.AWS.OIDCIssuer.URL != "" && c.AWS.OIDCIssuer.JWKSURI != "" {
		kubeAPIServerArgs = append(
			kubeAPIServerArgs,
			"service-account-issuer="+c.AWS.OIDCIssuer.URL,
			"service-account-issuer=https://kubernetes.default.svc."+c.Kubernetes.Domain,
			"service-account-jwks-uri="+c.AWS.OIDCIssuer.JWKSURI,
		)
	}
	return yaml.Marshal(config{
		ClusterCIDR:       c.Kubernetes.Subnets.Pods,
		ServiceCIDR:       c.Kubernetes.Subnets.Services,
		ClusterDNS:        c.Kubernetes.DNS,
		ClusterDomain:     c.Kubernetes.Domain,
		TLSSAN:            []string{c.Kubernetes.API},
		KubeAPIServerArgs: kubeAPIServerArgs,
	})
}

func BootstrapConfig(in BootstrapInput) ([]byte, error) {
	labels, err := mergeValues(autoLabels(in.ImageMetadata), in.Labels, "label", labelKey)
	if err != nil {
		return nil, err
	}
	taints, err := mergeValues(autoControlPlaneTaints(), in.Taints, "taint", taintKey)
	if err != nil {
		return nil, err
	}

	type config struct {
		ClusterInit bool     `yaml:"cluster-init"`
		NodeName    string   `yaml:"node-name"`
		NodeLabel   []string `yaml:"node-label,omitempty"`
		NodeTaint   []string `yaml:"node-taint,omitempty"`
	}
	return yaml.Marshal(config{
		ClusterInit: true,
		NodeName:    in.NodeName,
		NodeLabel:   labels,
		NodeTaint:   taints,
	})
}

func JoinConfig(in JoinInput) ([]byte, error) {
	labels, err := mergeValues(autoLabels(in.ImageMetadata), in.Labels, "label", labelKey)
	if err != nil {
		return nil, err
	}
	taints := in.Taints
	if in.ControlPlane {
		taints = append(autoControlPlaneTaints(), taints...)
	}
	taints, err = mergeValues(nil, taints, "taint", taintKey)
	if err != nil {
		return nil, err
	}

	type config struct {
		Server    string   `yaml:"server"`
		Token     string   `yaml:"token"`
		NodeName  string   `yaml:"node-name"`
		NodeLabel []string `yaml:"node-label,omitempty"`
		NodeTaint []string `yaml:"node-taint,omitempty"`
	}
	return yaml.Marshal(config{
		Server:    in.ServerURL,
		Token:     in.Token,
		NodeName:  in.NodeName,
		NodeLabel: labels,
		NodeTaint: taints,
	})
}

func ServerActivationCloudConfig(hostname string, operatorKeys []string) []byte {
	type config struct {
		Name   string           `yaml:"name"`
		Stages activationStages `yaml:"stages"`
		K3s    struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"k3s"`
	}
	out := config{
		Name:   "K2 K3s server activation",
		Stages: hostnameStages(hostname),
	}
	_ = operatorKeys
	out.K3s.Enabled = true

	return mustCloudConfig(out)
}

func AgentActivationCloudConfig(hostname string, operatorKeys []string) []byte {
	type config struct {
		Name     string           `yaml:"name"`
		Stages   activationStages `yaml:"stages"`
		K3sAgent struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"k3s-agent"`
	}
	out := config{
		Name:   "K2 K3s worker activation",
		Stages: hostnameStages(hostname),
	}
	_ = operatorKeys
	out.K3sAgent.Enabled = true

	return mustCloudConfig(out)
}

func ActivationCloudConfig(hostname string, operatorKeys []string) []byte {
	return ServerActivationCloudConfig(hostname, operatorKeys)
}

func HostnameActivationCloudConfig(name string, hostname string) []byte {
	type config struct {
		Name   string           `yaml:"name"`
		Stages activationStages `yaml:"stages"`
	}
	out := config{
		Name:   name,
		Stages: hostnameStages(hostname),
	}
	return mustCloudConfig(out)
}

func hostnameStages(hostname string) activationStages {
	return activationStages{
		Initramfs: []activationStage{
			{
				Name:     "Set local hostname",
				Hostname: hostname,
				// /etc is ephemeral; without this entry every sudo warns
				// "unable to resolve host <hostname>".
				Commands: []string{hostsEntryCommand(hostname)},
			},
		},
	}
}

func hostsEntryCommand(hostname string) string {
	entry := "127.0.1.1 " + hostname
	return fmt.Sprintf("grep -qxF %q /etc/hosts || echo %q >> /etc/hosts", entry, entry)
}

// OperatorKeysActivationCloudConfig renders an /oem stage that installs
// operator SSH keys on every boot, including recovery boots. Keys under
// /home/<user> alone do not survive `kairos-agent reset` (COS_PERSISTENT is
// wiped) while hardening's removal of the default password DOES survive
// (it edits /oem) — without this stage a hardened node comes out of reset
// unreachable over SSH.
func OperatorKeysActivationCloudConfig(name string, user string, keys []string) []byte {
	type config struct {
		Name   string           `yaml:"name"`
		Stages activationStages `yaml:"stages"`
	}
	// A full users: definition (admin group, key-only) rather than bare
	// authorized_keys: kairos-agent 2.30+/Kairos 3.3+ refuses reset/install
	// unless some applied config defines an admin-group user — and hardening
	// disables the stock 90_custom.yaml that used to.
	out := config{
		Name: name,
		Stages: activationStages{
			Initramfs: []activationStage{
				{
					Name: "Operator user and keys",
					Users: map[string]activationUser{
						user: {
							Groups:            []string{"admin"},
							SSHAuthorizedKeys: keys,
						},
					},
				},
			},
		},
	}
	return mustCloudConfig(out)
}

func mustCloudConfig(value any) []byte {
	data, err := yaml.Marshal(value)
	if err != nil {
		panic(err)
	}
	return []byte("#cloud-config\n" + string(data))
}

func AuthorizedKeys(keys []string) []byte {
	return []byte(strings.Join(keys, "\n") + "\n")
}

func DecodeImageMetadata(data []byte) (ImageMetadata, error) {
	var metadata ImageMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return ImageMetadata{}, fmt.Errorf("parse image metadata: %w", err)
	}
	return metadata, nil
}

func autoLabels(metadata ImageMetadata) []string {
	var values []string
	if metadata.Hardware != "" {
		values = append(values, LabelHardware+"="+metadata.Hardware)
	}
	if metadata.Target != "" {
		values = append(values, LabelImageTarget+"="+metadata.Target)
	}
	if metadata.Arch != "" {
		values = append(values, LabelArch+"="+metadata.Arch)
	}
	return values
}

func autoControlPlaneTaints() []string {
	return []string{
		"node-role.kubernetes.io/control-plane=true:NoSchedule",
	}
}

func mergeValues(auto []string, custom []string, kind string, keyFunc func(string) (string, error)) ([]string, error) {
	merged := map[string]string{}
	for _, value := range append(auto, custom...) {
		key, err := keyFunc(value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", kind, value, err)
		}
		if existing, ok := merged[key]; ok && existing != value {
			return nil, fmt.Errorf("conflicting %s values for %q: %q and %q", kind, key, existing, value)
		}
		merged[key] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, merged[key])
	}
	return out, nil
}

func labelKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}
	before, _, found := strings.Cut(value, "=")
	if !found {
		return "", fmt.Errorf("expected key=value form")
	}
	if strings.TrimSpace(before) == "" {
		return "", fmt.Errorf("key is empty")
	}
	if err := validateKubeletLabelKey(before); err != nil {
		return "", err
	}
	return before, nil
}

func validateKubeletLabelKey(key string) error {
	prefix, _, found := strings.Cut(key, "/")
	if !found {
		return nil
	}
	switch prefix {
	case "kubelet.kubernetes.io", "node.kubernetes.io":
		return nil
	case "kubernetes.io", "k8s.io":
		if allowedKubeletReservedLabel(key) {
			return nil
		}
		return fmt.Errorf("reserved kubelet label %q is not allowed in node-label; apply it through the Kubernetes API after node registration", key)
	default:
		if strings.HasSuffix(prefix, ".kubernetes.io") || strings.HasSuffix(prefix, ".k8s.io") {
			return fmt.Errorf("reserved kubelet label %q is not allowed in node-label; apply it through the Kubernetes API after node registration", key)
		}
		return nil
	}
}

func allowedKubeletReservedLabel(key string) bool {
	switch key {
	case "beta.kubernetes.io/arch",
		"beta.kubernetes.io/instance-type",
		"beta.kubernetes.io/os",
		"failure-domain.beta.kubernetes.io/region",
		"failure-domain.beta.kubernetes.io/zone",
		"kubernetes.io/arch",
		"kubernetes.io/hostname",
		"kubernetes.io/os",
		"node.kubernetes.io/instance-type",
		"topology.kubernetes.io/region",
		"topology.kubernetes.io/zone":
		return true
	default:
		return false
	}
}

func taintKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}
	keyValue, effect, found := strings.Cut(value, ":")
	if !found {
		return "", fmt.Errorf("expected key[=value]:effect form")
	}
	if strings.TrimSpace(effect) == "" {
		return "", fmt.Errorf("effect is empty")
	}
	key, _, _ := strings.Cut(keyValue, "=")
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("key is empty")
	}
	return key, nil
}

func WriteDryRunFile(buf *bytes.Buffer, path string, data []byte) {
	fmt.Fprintf(buf, "--- %s ---\n", path)
	buf.Write(data)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		buf.WriteByte('\n')
	}
}
