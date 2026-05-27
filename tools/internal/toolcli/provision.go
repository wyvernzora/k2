package toolcli

type provisionCmd struct {
	Bootstrap bootstrapCmd `cmd:"" help:"Provision the first K3s server over SSH."`
	Server    serverCmd    `cmd:"" help:"Provision an additional K3s server over SSH."`
	Worker    workerCmd    `cmd:"" help:"Provision a K3s worker over SSH."`
	Render    renderCmd    `cmd:"" help:"Render provisioning files locally."`
}

type renderCmd struct {
	Bootstrap renderBootstrapCmd `cmd:"" help:"Render bootstrap provisioning files."`
}

type commonBootstrapFlags struct {
	ClusterTarget    string   `name:"cluster-target" env:"K2_PROVISION_CLUSTER_TARGET" required:"" help:"Cluster config/deploy target, such as v3."`
	ClusterName      string   `name:"cluster-name" env:"K2_PROVISION_CLUSTER_NAME" help:"Local cluster instance name. Defaults to cluster-target."`
	NodeName         string   `name:"node-name" env:"K2_PROVISION_NODE_NAME" help:"Kubernetes node name. Defaults to --test-vm when provided."`
	OperatorKey      []string `name:"operator-key" env:"K2_PROVISION_OPERATOR_KEY" help:"Literal ssh-ed25519 operator public key. Repeatable."`
	OperatorFiles    []string `name:"operator-key-file" env:"K2_PROVISION_OPERATOR_KEY_FILE" help:"File containing literal operator public keys. Repeatable." type:"path"`
	Label            []string `name:"label" env:"K2_PROVISION_LABEL" help:"Additional K3s node-label value. Repeatable."`
	Taint            []string `name:"taint" env:"K2_PROVISION_TAINT" help:"Additional K3s node-taint value. Repeatable."`
	BootstrapAPIHost string   `name:"bootstrap-api-host" env:"K2_PROVISION_BOOTSTRAP_API_HOST" help:"Kubernetes API host for bootstrap-only Cilium manifests. Bootstrap provisioning auto-detects the node IP when omitted."`
	ExtraManifests   []string `name:"extra-manifests" env:"K2_PROVISION_EXTRA_MANIFESTS" help:"Extra bootstrap manifest path or glob to append verbatim. Repeatable."`

	testKubeVIP string
}

type bootstrapCmd struct {
	commonBootstrapFlags

	TestVM   string `name:"test-vm" env:"K2_PROVISION_TEST_VM" help:"Provision the local test VM id, defaulting host and cluster-name for VM swarm tests."`
	Host     string `name:"host" env:"K2_PROVISION_HOST" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" env:"K2_PROVISION_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" env:"K2_PROVISION_SSH_USER" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" env:"K2_PROVISION_NO_REBOOT" help:"Install files and enable k3s, but do not reboot."`
	Yes      bool   `name:"yes" env:"K2_PROVISION_YES" help:"Skip the plan confirmation prompt. Required for non-TTY invocations."`
}

type commonJoinFlags struct {
	ClusterTarget string   `name:"cluster-target" env:"K2_PROVISION_CLUSTER_TARGET" required:"" help:"Cluster config/deploy target, such as v3."`
	ClusterName   string   `name:"cluster-name" env:"K2_PROVISION_CLUSTER_NAME" help:"Local cluster instance name. Defaults to cluster-target."`
	NodeName      string   `name:"node-name" env:"K2_PROVISION_NODE_NAME" help:"Kubernetes node name. Defaults to --test-vm when provided."`
	OperatorKey   []string `name:"operator-key" env:"K2_PROVISION_OPERATOR_KEY" help:"Literal ssh-ed25519 operator public key. Repeatable."`
	OperatorFiles []string `name:"operator-key-file" env:"K2_PROVISION_OPERATOR_KEY_FILE" help:"File containing literal operator public keys. Repeatable." type:"path"`
	Label         []string `name:"label" env:"K2_PROVISION_LABEL" help:"Additional K3s node-label value. Repeatable."`
	Taint         []string `name:"taint" env:"K2_PROVISION_TAINT" help:"Additional K3s node-taint value. Repeatable."`
	ServerURL     string   `name:"server-url" env:"K2_PROVISION_SERVER_URL" help:"K3s API URL for joining. Defaults to ~/.kube/k2/<cluster-name>/server-url, then the API VIP from clusters/<target>.yaml."`
}

type commonRemoteFlags struct {
	TestVM   string `name:"test-vm" env:"K2_PROVISION_TEST_VM" help:"Provision the local test VM id, defaulting host and cluster-name for VM swarm tests."`
	Host     string `name:"host" env:"K2_PROVISION_HOST" help:"SSH host for the clean Kairos node."`
	SSHPort  int    `name:"ssh-port" env:"K2_PROVISION_SSH_PORT" default:"22" help:"SSH port."`
	SSHUser  string `name:"ssh-user" env:"K2_PROVISION_SSH_USER" default:"kairos" help:"SSH user."`
	NoReboot bool   `name:"no-reboot" env:"K2_PROVISION_NO_REBOOT" help:"Install files and enable k3s, but do not reboot."`
	Yes      bool   `name:"yes" env:"K2_PROVISION_YES" help:"Skip the plan confirmation prompt. Required for non-TTY invocations."`
}

type serverCmd struct {
	commonJoinFlags
	commonRemoteFlags
}

type workerCmd struct {
	commonJoinFlags
	commonRemoteFlags
}

type renderBootstrapCmd struct {
	commonBootstrapFlags

	OutputDir string `name:"output-dir" env:"K2_PROVISION_OUTPUT_DIR" required:"" help:"Directory to write rendered files into." type:"path"`
}

type bundle struct {
	ClusterConfig   []byte
	BootstrapConfig []byte
	Activation      []byte
	AuthorizedKeys  []byte
	Manifests       []byte
	RootArgoApp     []byte
}

type joinBundle struct {
	ClusterConfig  []byte
	JoinConfig     []byte
	Activation     []byte
	AuthorizedKeys []byte
}

type nodeRole string

type testVMProvisionTarget struct {
	Enabled bool
	GuestIP string
	KubeVIP string
}

const (
	nodeRoleServer nodeRole = "server"
	nodeRoleWorker nodeRole = "worker"

	longhornStorageNodeLabel          = "node.longhorn.io/create-default-disk=true"
	longhornStorageNodeTag            = "k2-storage"
	longhornStorageNodeTagsAnnotation = `node.longhorn.io/default-node-tags=["k2-storage"]`
	longhornNodeLabelPrefix           = "node.longhorn.io/"

	remoteRootArgoAppManifestPath = "/var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml"
)
