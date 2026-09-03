package provision

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/keys"
	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
	"github.com/wyvernzora/k2/tools/internal/render"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

const storageRole = "storage"

type storageState struct {
	client          *remote.Client
	metadata        render.ImageMetadata
	inspection      storageInspection
	vdevs           []storageVDev
	poolPlan        storagePoolPlan
	bundle          storageBundle
	localDir        string
	remoteDir       string
	csiPublicKey    string
	csiPrivateKey   string
	csiKeyGenerated bool
	chapUsername    string
	chapPassword    string
	poolKey         string
	credentialsPath string
	summary         storageSummary
}

type storageBundle struct {
	Activation         []byte
	AuthorizedKeys     []byte
	OperatorActivation []byte
	CSIPublicKey       []byte
	BackupKeys         []byte // nil when no backup keys are supplied
	SnapshotEnv        []byte
	PoolKey            []byte
	InstallScript      []byte
	PoolScript         []byte
	Network            []byte // nil when the node file declares no NICs
}

type storageCredentials struct {
	Portal                             string `json:"portal"`
	IQNBase                            string `json:"iqnBase"`
	Pool                               string `json:"pool"`
	DatasetParentName                  string `json:"datasetParentName"`
	DetachedSnapshotsDatasetParentName string `json:"detachedSnapshotsDatasetParentName"`
	SSHHost                            string `json:"sshHost"`
	SSHPort                            int    `json:"sshPort"`
	SSHUser                            string `json:"sshUser"`
	CSIPrivateKey                      string `json:"csiPrivateKey,omitempty"`
	CSIPublicKey                       string `json:"csiPublicKey"`
	CHAPUsername                       string `json:"chapUsername"`
	CHAPPassword                       string `json:"chapPassword"`
	PoolKey                            string `json:"poolKey,omitempty"`
	ProvisionedAt                      string `json:"provisionedAt"`
}

func (c storageCredentials) summary(path string) storageSummary {
	return storageSummary{
		Portal:                             c.Portal,
		IQNBase:                            c.IQNBase,
		Pool:                               c.Pool,
		DatasetParentName:                  c.DatasetParentName,
		DetachedSnapshotsDatasetParentName: c.DetachedSnapshotsDatasetParentName,
		SSHHost:                            c.SSHHost,
		SSHPort:                            c.SSHPort,
		SSHUser:                            c.SSHUser,
		CSIPublicKey:                       c.CSIPublicKey,
		CHAPUsername:                       c.CHAPUsername,
		CredentialsFile:                    path,
		ProvisionedAt:                      c.ProvisionedAt,
	}
}

type storageSummary struct {
	Portal                             string `json:"portal"`
	IQNBase                            string `json:"iqnBase"`
	Pool                               string `json:"pool"`
	DatasetParentName                  string `json:"datasetParentName"`
	DetachedSnapshotsDatasetParentName string `json:"detachedSnapshotsDatasetParentName"`
	SSHHost                            string `json:"sshHost"`
	SSHPort                            int    `json:"sshPort"`
	SSHUser                            string `json:"sshUser"`
	CSIPublicKey                       string `json:"csiPublicKey"`
	CHAPUsername                       string `json:"chapUsername"`
	CredentialsFile                    string `json:"credentialsFile"`
	ProvisionedAt                      string `json:"provisionedAt"`
}

func (c *storageCmd) Run(rcx *Runtime) error {
	state, err := runStorageProvision(context.Background(), rcx, c)
	if err != nil {
		return err
	}
	if c.Output == "json" {
		data, err := json.MarshalIndent(state.summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
	}
	return nil
}

func runStorageProvision(parent context.Context, rcx *Runtime, c *storageCmd) (*storageState, error) {
	if err := c.prepare(rcx); err != nil {
		return nil, err
	}
	state, err := c.newStorageState()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	reporter := currentReporter()
	prevCancel := reporter.SetInterruptCancel(cancel)
	defer func() {
		reporter.SetInterruptCancel(prevCancel)
	}()

	wf := ui.NewWorkflow(currentReporter())
	c.buildStorageWorkflow(wf, state)
	if err := wf.Execute(ctx); err != nil {
		return nil, err
	}
	return state, nil
}

func (c *storageCmd) prepare(rcx *Runtime) error {
	if _, err := applyProvisionTestVM(rcx.RepoRoot, c.ClusterTarget, &c.ClusterName, &c.NodeName, &c.Host, &c.SSHPort, c.TestVM); err != nil {
		return err
	}
	if c.ClusterName == "" {
		c.ClusterName = c.ClusterTarget
	}
	if c.NodeName == "" {
		c.NodeName = "k2-storage"
	}
	node, nodeFileFound, err := nodeconfig.Load(rcx.RepoRoot, c.ClusterTarget, c.NodeName)
	if err != nil {
		return err
	}
	if nodeFileFound {
		logf("loaded node config %s", nodeconfig.Path(rcx.RepoRoot, c.ClusterTarget, c.NodeName))
	}
	c.node = node
	if c.Portal == "" {
		// A static appliance advertises its pinned primary address; the
		// bootstrap --host address is not guaranteed to exist after reboot.
		if addr := node.PrimaryAddress(); addr != "" {
			c.Portal = addr + ":3260"
		} else {
			c.Portal = c.Host + ":3260"
		}
	}
	_, err = parseStorageVDevs(c.PoolVDev, c.TestVM != "")
	return err
}

func (c *storageCmd) newStorageState() (*storageState, error) {
	existing, haveExisting, err := loadStorageCredentials(c.ClusterName)
	if err != nil {
		return nil, err
	}
	// Re-provisioning (the disaster-recovery drill: reset → provision) must
	// restore the SAME csi key and CHAP credentials the cluster already
	// holds, so an existing credentials file is reused unless the operator
	// explicitly rotates or supplies a key.
	if !c.RotateCredentials && c.CSIPublicKey == "" {
		if haveExisting {
			if existing.PoolKey == "" {
				var err error
				existing.PoolKey, err = generatePoolKey()
				if err != nil {
					return nil, err
				}
			}
			logf("reusing csi key and CHAP credentials from existing %s (pass --rotate-credentials to regenerate)", "storage-appliance.json")
			return c.storageStateFromCredentials(existing), nil
		}
	}
	pub, priv, generated, err := resolveCSIKey(c.CSIPublicKey)
	if err != nil {
		return nil, err
	}
	chapPassword, err := randomBase62(16)
	if err != nil {
		return nil, err
	}
	poolKey := ""
	if haveExisting {
		// Pool wrapping keys outlive SSH/CHAP material; rotating this silently
		// would strand an encrypted pool.
		poolKey = existing.PoolKey
	}
	if poolKey == "" {
		poolKey, err = generatePoolKey()
		if err != nil {
			return nil, err
		}
	}
	return &storageState{
		client: &remote.Client{
			Host:             c.Host,
			Port:             c.SSHPort,
			User:             c.SSHUser,
			IdentityFile:     c.Identity,
			InsecureHostKey:  c.TestVM != "",
			NoPasswordPrompt: c.noPasswordPrompt,
			Stdout:           os.Stdout,
			Stderr:           os.Stderr,
			Logger:           logf,
		},
		csiPublicKey:    pub,
		csiPrivateKey:   priv,
		csiKeyGenerated: generated,
		chapUsername:    "k2-" + c.ClusterName,
		chapPassword:    chapPassword,
		poolKey:         poolKey,
	}, nil
}

func (c *storageCmd) storageStateFromCredentials(creds storageCredentials) *storageState {
	return &storageState{
		client: &remote.Client{
			Host:             c.Host,
			Port:             c.SSHPort,
			User:             c.SSHUser,
			IdentityFile:     c.Identity,
			InsecureHostKey:  c.TestVM != "",
			NoPasswordPrompt: c.noPasswordPrompt,
			Stdout:           os.Stdout,
			Stderr:           os.Stderr,
			Logger:           logf,
		},
		csiPublicKey:  creds.CSIPublicKey,
		csiPrivateKey: creds.CSIPrivateKey,
		chapUsername:  creds.CHAPUsername,
		chapPassword:  creds.CHAPPassword,
		poolKey:       creds.PoolKey,
	}
}

// loadStorageCredentials returns (creds, true, nil) when a valid file
// exists, (zero, false, nil) when none exists, and a non-nil error when a
// file EXISTS but cannot be used. The error case must abort, not fall back
// to regeneration: silently rotating credentials on a corrupt file also
// rotates the pool wrapping key of an existing encrypted pool.
func loadStorageCredentials(clusterName string) (storageCredentials, bool, error) {
	dir, err := clusterCredentialsDir(clusterName)
	if err != nil {
		return storageCredentials{}, false, err
	}
	path := filepath.Join(dir, "storage-appliance.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return storageCredentials{}, false, nil
	}
	if err != nil {
		return storageCredentials{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	var creds storageCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return storageCredentials{}, false, fmt.Errorf("%s exists but is not valid JSON (%w); refusing to regenerate credentials over it", path, err)
	}
	if creds.CSIPublicKey == "" || creds.CHAPUsername == "" || creds.CHAPPassword == "" {
		return storageCredentials{}, false, fmt.Errorf("%s exists but is missing required fields; refusing to regenerate credentials over it", path)
	}
	return creds, true, nil
}

func (c *storageCmd) buildStorageWorkflow(wf *ui.Workflow, s *storageState) {
	wf.Section("Plan")
	wf.Shell("Read remote image metadata", c.stepStorageReadMetadata(s))
	wf.Shell("Inspect storage state", c.stepStorageInspect(s))
	wf.Task("Resolve storage plan", c.stepStorageResolvePlan(s))
	wf.KeyValuesFn(func() []ui.KV { return c.storagePlanFields(s) })
	wf.TableFn([]string{"DISK", "SIZE", "MODEL", "STATE"}, func() [][]string { return storageDiskRows(s.inspection.Disks) }).
		When(func() bool { return len(s.inspection.Disks) > 0 })
	wf.Confirm("Proceed with provisioning? [y/N]", "").Unless(c.Yes)

	wf.Section("Render bundle")
	wf.Task("Render storage bundle", c.stepRenderStorageBundle(s))
	wf.Task("Stage bundle locally", c.stepStageStorageBundle(wf, s))
	wf.KeyValuesFn(func() []ui.KV { return []ui.KV{{Key: "Staging dir", Value: s.localDir}} })

	wf.Section("Provision storage")
	// Credentials (including the pool wrapping key) are escrowed BEFORE the
	// pool exists: if provisioning dies anywhere after zpool create, the key
	// that encrypted the pool must already be on disk locally, or a rerun
	// would generate a fresh key for a pool it can never unlock again.
	wf.Task("Escrow pool key and credentials", c.stepWriteStorageCredentials(s))
	wf.Shell("Upload storage bundle to remote", c.stepUploadStorageBundle(s))
	wf.Shell("Install hostname and users", c.stepRunStorageInstall(s))
	wf.Shell("Provision ZFS pool and datasets", c.stepRunStoragePool(s))
	wf.Shell("Run storage health check", c.stepStorageHealth(s))
	wf.Shell("Harden default access", c.stepStorageHarden(s))
	wf.Task("Write local storage credentials", c.stepWriteStorageCredentials(s))

	wf.BannerFn(ui.BannerSuccess, func() []string { return c.storageBanner(s) }).
		Unless(c.Output == "json")
}

func (c *storageCmd) stepStorageReadMetadata(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		s.metadata, err = readRemoteMetadata(s.client)
		if err != nil {
			return fmt.Errorf("%w; rebuild the image with baked metadata support", err)
		}
		if s.metadata.Role != storageRole {
			return fmt.Errorf("remote image role is %q, want %q", s.metadata.Role, storageRole)
		}
		// Refuse recovery/autoreset boots: /oem is not writable there and
		// anything provisioned would not survive the pending reset.
		out, err := s.client.Capture("if [ -e /run/cos/recovery_mode ] || [ -e /run/cos/autoreset_mode ]; then echo recovery; else echo active; fi")
		if err != nil {
			return fmt.Errorf("detect boot mode: %w", err)
		}
		if strings.TrimSpace(string(out)) != "active" {
			return fmt.Errorf("appliance is in recovery/autoreset boot; wait for the installed system to come up and re-run")
		}
		sh.Successf("image %s %s %s role=%s", s.metadata.Target, s.metadata.Arch, s.metadata.Hardware, s.metadata.Role)
		return nil
	}
}

func (c *storageCmd) stepStorageInspect(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		s.inspection, err = inspectRemoteStorage(s.client, c.Pool)
		if err != nil {
			return err
		}
		sh.Successf("pool state %s; %d candidate disk(s)", s.inspection.PoolState, len(s.inspection.Disks))
		return nil
	}
}

func (c *storageCmd) stepStorageResolvePlan(s *storageState) func(context.Context) error {
	return func(ctx context.Context) error {
		vdevs, err := parseStorageVDevs(c.PoolVDev, c.TestVM != "")
		if err != nil {
			return err
		}
		if len(vdevs) == 0 && s.inspection.PoolState == storagePoolMissing {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("no pool present; pass --pool-vdev or run interactively")
			}
			vdevs, err = promptStorageVDevs(os.Stdin, os.Stderr, s.inspection.Disks, c.ForceWipe)
			if err != nil {
				return err
			}
		}
		// Scenario/CLI invocations pass the same vdevs on every run, but a
		// reused appliance already has the pool. Ignore them instead of
		// erroring so re-provisioning stays a plain rerun.
		if s.inspection.PoolState != storagePoolMissing && len(vdevs) > 0 {
			logf("pool %s already exists; ignoring --pool-vdev", c.Pool)
			vdevs = nil
		}
		s.vdevs = vdevs
		s.poolPlan, err = resolveStoragePoolPlan(c.Pool, s.inspection, vdevs)
		return err
	}
}

func (c *storageCmd) stepRenderStorageBundle(s *storageState) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		s.bundle, err = buildStorageBundle(c.commonStorageFlags, c.node, c.ForceWipe, s.vdevs, s.csiPublicKey, s.poolKey)
		return err
	}
}

func (c *storageCmd) stepStageStorageBundle(wf *ui.Workflow, s *storageState) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		s.localDir, err = os.MkdirTemp("", "k2-tools-storage-*")
		if err != nil {
			return err
		}
		wf.Defer(func() { _ = os.RemoveAll(s.localDir) })
		return writeStorageBundle(s.localDir, s.bundle)
	}
}

func (c *storageCmd) stepUploadStorageBundle(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		var err error
		s.remoteDir, err = s.client.UploadDir(s.localDir)
		if err != nil {
			return err
		}
		sh.Successf("uploaded to %s", s.remoteDir)
		return nil
	}
}

func (c *storageCmd) stepRunStorageInstall(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return s.client.Run("sh " + shellQuote(s.remoteDir+"/storage-install.sh"))
	}
}

func (c *storageCmd) stepRunStoragePool(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return s.client.Run("sh " + shellQuote(s.remoteDir+"/storage-pool.sh"))
	}
}

func (c *storageCmd) stepStorageHealth(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		out, err := s.client.Capture("sudo k2-node-agent storage-health")
		if err != nil {
			return fmt.Errorf("storage health check: %w", err)
		}
		line := firstNonEmptyLine(out)
		if line == "" {
			line = "storage health passed"
		}
		sh.Successf("%s", line)
		return nil
	}
}

func (c *storageCmd) stepStorageHarden(s *storageState) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		defer s.client.SwapIO(sh)()
		return hardenRemoteDefaultAccess(s.client)
	}
}

func (c *storageCmd) stepWriteStorageCredentials(s *storageState) func(context.Context) error {
	return func(ctx context.Context) error {
		path, summary, err := c.writeStorageCredentials(s)
		if err != nil {
			return err
		}
		s.credentialsPath = path
		s.summary = summary
		return nil
	}
}

func (c *storageCmd) writeStorageCredentials(s *storageState) (string, storageSummary, error) {
	dir, err := clusterCredentialsDir(c.ClusterName)
	if err != nil {
		return "", storageSummary{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", storageSummary{}, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", storageSummary{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	creds := storageCredentials{
		Portal:                             c.Portal,
		IQNBase:                            c.IQNBase,
		Pool:                               c.Pool,
		DatasetParentName:                  c.datasetParent(),
		DetachedSnapshotsDatasetParentName: c.snapshotsParent(),
		SSHHost:                            c.Host,
		SSHPort:                            c.SSHPort,
		SSHUser:                            "k2-csi",
		CSIPrivateKey:                      s.csiPrivateKey,
		CSIPublicKey:                       s.csiPublicKey,
		CHAPUsername:                       s.chapUsername,
		CHAPPassword:                       s.chapPassword,
		PoolKey:                            s.poolKey,
		ProvisionedAt:                      now,
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return "", storageSummary{}, err
	}
	path := filepath.Join(dir, "storage-appliance.json")
	// Atomic replace: a crash mid-write must never leave corrupt JSON — an
	// unreadable credentials file is what triggers pool-key regeneration.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return "", storageSummary{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", storageSummary{}, err
	}
	return path, creds.summary(path), nil
}

func (c *storageCmd) storagePlanFields(s *storageState) []ui.KV {
	keyStatus := "provided"
	if s.csiKeyGenerated {
		keyStatus = "new local ed25519 keypair"
	}
	if s.csiPrivateKey != "" && !s.csiKeyGenerated {
		keyStatus = "reused from credentials file"
	}
	chapStatus := "new credentials"
	if !s.csiKeyGenerated && s.csiPrivateKey != "" {
		chapStatus = "reused from credentials file"
	}
	return []ui.KV{
		{Key: "Cluster target", Value: c.ClusterTarget},
		{Key: "Cluster name", Value: c.ClusterName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", c.SSHUser, c.Host, c.SSHPort)},
		{Key: "Image", Value: fmt.Sprintf("%s %s %s role=%s", s.metadata.Target, s.metadata.Arch, s.metadata.Hardware, s.metadata.Role)},
		{Key: "Node name", Value: c.NodeName},
		{Key: "Pool", Value: s.poolPlan.String()},
		{Key: "Pool options", Value: "ashift=12, autotrim=on, compatibility=" + c.PoolCompatibility},
		{Key: "Encryption", Value: "aes-256-gcm (raw key on persistent partition, escrowed in credentials file)"},
		{Key: "Dataset parent", Value: c.datasetParent()},
		{Key: "Detached snapshots", Value: c.snapshotsParent()},
		{Key: "CSI user/key", Value: "csi, " + keyStatus},
		{Key: "CHAP", Value: chapStatus},
		{Key: "Hardening", Value: "kairos password auth will be disabled"},
		{Key: "Reboot", Value: "not required"},
	}
}

func (c *storageCmd) storageBanner(s *storageState) []string {
	return []string{
		"Storage provisioning complete",
		"Portal: " + c.Portal,
		"IQN base: " + c.IQNBase,
		"Datasets: " + c.datasetParent() + ", " + c.snapshotsParent(),
		"CSI user: csi",
		"Credentials: " + s.credentialsPath,
	}
}

func (c *storageCmd) datasetParent() string {
	return c.Pool + "/csi/" + c.ClusterName
}

func (c *storageCmd) snapshotsParent() string {
	return c.Pool + "/csi/" + c.ClusterName + "-snapshots"
}

func buildStorageBundle(flags commonStorageFlags, node nodeconfig.Config, forceWipe bool, vdevs []storageVDev, csiPublicKey, poolKey string) (storageBundle, error) {
	operatorKeys, err := keys.Load(flags.OperatorKey, flags.OperatorFiles)
	if err != nil {
		return storageBundle{}, err
	}
	rawPoolKey, err := decodePoolKey(poolKey)
	if err != nil {
		return storageBundle{}, err
	}
	backupKeys, err := loadOptionalOperatorKeys(flags.BackupKey, flags.BackupKeyFiles)
	if err != nil {
		return storageBundle{}, err
	}
	// Design D7: targetcli requires root; the csi key is treated as a root credential.

	bundle := storageBundle{
		Activation:         render.HostnameActivationCloudConfig("K2 storage hostname activation", flags.NodeName),
		AuthorizedKeys:     render.AuthorizedKeys(operatorKeys),
		OperatorActivation: render.OperatorKeysActivationCloudConfig("K2 storage operator keys", "kairos", operatorKeys),
		CSIPublicKey:       []byte(strings.TrimSpace(csiPublicKey) + "\n"),
		SnapshotEnv:        []byte(snapshotEnv(flags)),
		PoolKey:            rawPoolKey,
	}
	if len(backupKeys) > 0 {
		bundle.BackupKeys = render.AuthorizedKeys(backupKeys)
	}
	if len(node.NICs) > 0 {
		// Stage-only: the static addresses apply on the appliance's next
		// boot. Applying them live would drop the SSH session mid-provision;
		// the flow already requires a reboot before the appliance serves
		// (D26 boot-chain verification).
		bundle.Network = render.NetworkActivationCloudConfig(node.NICs)
	}
	bundle.InstallScript = []byte(storageInstallScript(flags.NodeName, len(node.NICs) > 0, len(backupKeys) > 0))
	bundle.PoolScript = []byte(storagePoolScript(storagePoolScriptInput{
		Pool:          flags.Pool,
		ClusterName:   flags.ClusterName,
		Compatibility: flags.PoolCompatibility,
		VDevs:         vdevs,
		ForceWipe:     forceWipe,
		CreateAllowed: len(vdevs) > 0,
	}))
	return bundle, nil
}

// Cadence retention defaults, mirrored by the --snapshot-*-keep kong tags and
// by the storage overlay's unit Environment= fallbacks.
const (
	defaultSnapshotHourlyKeep = 48
	defaultSnapshotDailyKeep  = 30
)

func snapshotEnv(flags commonStorageFlags) string {
	dataset := flags.Pool + "/csi/" + clusterNameOrFallback(flags.ClusterName, flags.ClusterTarget)
	return fmt.Sprintf(
		"K2_SNAPSHOT_DATASET=%s\nK2_SNAPSHOT_HOURLY_KEEP=%d\nK2_SNAPSHOT_DAILY_KEEP=%d\n",
		dataset,
		snapshotKeep(flags.SnapshotHourly, defaultSnapshotHourlyKeep),
		snapshotKeep(flags.SnapshotDaily, defaultSnapshotDailyKeep),
	)
}

// snapshotKeep substitutes the default for a non-positive retention: kong fills
// the flags in, but programmatic callers (StorageInputs) leave the fields at
// their zero value, and k2-node-agent rejects keep < 1 — a rendered 0 would
// fail every timer tick instead of snapshotting.
func snapshotKeep(keep, fallback int) int {
	if keep < 1 {
		return fallback
	}
	return keep
}

func loadOptionalOperatorKeys(literal []string, files []string) ([]string, error) {
	if len(literal) == 0 && len(files) == 0 {
		return nil, nil
	}
	return keys.Load(literal, files)
}

func writeStorageBundle(dir string, bundle storageBundle) error {
	files := map[string][]byte{
		"99-k2-storage-hostname.yaml":      bundle.Activation,
		"98-k2-storage-operator-keys.yaml": bundle.OperatorActivation,
		"operator_authorized_keys":         bundle.AuthorizedKeys,
		"csi_authorized_keys":              bundle.CSIPublicKey,
		"zfs_pool.key":                     bundle.PoolKey,
		"storage-install.sh":               bundle.InstallScript,
		"storage-pool.sh":                  bundle.PoolScript,
		"97-k2-network.yaml":               bundle.Network,
		"backup_authorized_keys":           bundle.BackupKeys,
		"k2-snapshot.env":                  bundle.SnapshotEnv,
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if len(data) == 0 {
			continue
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		} else if strings.HasSuffix(name, ".key") {
			mode = 0o600
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
			return err
		}
	}
	return nil
}

func resolveCSIKey(value string) (publicKey string, privateKey string, generated bool, err error) {
	value = strings.TrimSpace(value)
	if value != "" {
		if err := validateEd25519PublicKey(value); err != nil {
			return "", "", false, err
		}
		return value, "", false, nil
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", false, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", false, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "k2 storage csi")
	if err != nil {
		return "", "", false, err
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))), string(pem.EncodeToMemory(block)), true, nil
}

func generatePoolKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func decodePoolKey(value string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode pool key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("pool key must decode to 32 bytes, got %d", len(raw))
	}
	return raw, nil
}

func validateEd25519PublicKey(value string) error {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(value)))
	if err != nil {
		return fmt.Errorf("csi public key must be a literal ssh-ed25519 public key: %w", err)
	}
	if pub.Type() != ssh.KeyAlgoED25519 {
		return fmt.Errorf("csi public key must be ssh-ed25519, got %s", pub.Type())
	}
	return nil
}

func randomBase62(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	max := big.NewInt(int64(len(alphabet)))
	buf := make([]byte, n)
	for i := range buf {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = alphabet[idx.Int64()]
	}
	return string(buf), nil
}

func firstNonEmptyLine(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return ""
}

func storageDiskRows(disks []storageDisk) [][]string {
	rows := make([][]string, len(disks))
	for i, disk := range disks {
		rows[i] = []string{strings.TrimPrefix(disk.ByID, "/dev/disk/by-id/"), humanBytes(disk.Size), disk.Model, string(disk.State)}
	}
	return rows
}

func humanBytes(size int64) string {
	if size <= 0 {
		return ""
	}
	const unit = 1000
	value := float64(size)
	suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	i := 0
	for value >= unit && i < len(suffixes)-1 {
		value /= unit
		i++
	}
	if i == 0 {
		return strconv.FormatInt(size, 10) + "B"
	}
	return fmt.Sprintf("%.1f%s", value, suffixes[i])
}
