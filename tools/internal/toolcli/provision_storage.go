package toolcli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/keys"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/remote"
	"github.com/wyvernzora/k2/tools/internal/kairos/tools/render"
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
	credentialsPath string
	summary         storageSummary
}

type storageBundle struct {
	Activation         []byte
	AuthorizedKeys     []byte
	OperatorActivation []byte
	CSIPublicKey       []byte
	CSISudoers         []byte
	InstallScript      []byte
	PoolScript         []byte
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

func (c *storageCmd) Run(rcx *runContext) error {
	if err := c.prepare(rcx); err != nil {
		return err
	}
	state, err := c.newStorageState()
	if err != nil {
		return err
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	reporter.SetInterruptCancel(cancel)
	defer reporter.SetInterruptCancel(nil)

	wf := ui.NewWorkflow(reporter)
	c.buildStorageWorkflow(wf, state)
	if err := wf.Execute(parent); err != nil {
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

func (c *storageCmd) prepare(rcx *runContext) error {
	if _, err := applyProvisionTestVM(rcx.repoRoot, c.ClusterTarget, &c.ClusterName, &c.NodeName, &c.Host, &c.SSHPort, c.TestVM); err != nil {
		return err
	}
	if c.ClusterName == "" {
		c.ClusterName = c.ClusterTarget
	}
	if c.NodeName == "" {
		c.NodeName = "k2-storage"
	}
	if c.Portal == "" {
		c.Portal = c.Host + ":3260"
	}
	_, err := parseStorageVDevs(c.PoolVDev, c.TestVM != "")
	return err
}

func (c *storageCmd) newStorageState() (*storageState, error) {
	// Re-provisioning (the disaster-recovery drill: reset → provision) must
	// restore the SAME csi key and CHAP credentials the cluster already
	// holds, so an existing credentials file is reused unless the operator
	// explicitly rotates or supplies a key.
	if !c.RotateCredentials && c.CSIPublicKey == "" {
		if existing, ok := loadStorageCredentials(c.ClusterName); ok {
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
	return &storageState{
		client: &remote.Client{
			Host:         c.Host,
			Port:         c.SSHPort,
			User:         c.SSHUser,
			IdentityFile: c.Identity,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
			Logger:       logf,
		},
		csiPublicKey:    pub,
		csiPrivateKey:   priv,
		csiKeyGenerated: generated,
		chapUsername:    "k2-" + c.ClusterName,
		chapPassword:    chapPassword,
	}, nil
}

func (c *storageCmd) storageStateFromCredentials(creds storageCredentials) *storageState {
	return &storageState{
		client: &remote.Client{
			Host:         c.Host,
			Port:         c.SSHPort,
			User:         c.SSHUser,
			IdentityFile: c.Identity,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
			Logger:       logf,
		},
		csiPublicKey:  creds.CSIPublicKey,
		csiPrivateKey: creds.CSIPrivateKey,
		chapUsername:  creds.CHAPUsername,
		chapPassword:  creds.CHAPPassword,
	}
}

func loadStorageCredentials(clusterName string) (storageCredentials, bool) {
	dir, err := clusterCredentialsDir(clusterName)
	if err != nil {
		return storageCredentials{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, "storage-appliance.json"))
	if err != nil {
		return storageCredentials{}, false
	}
	var creds storageCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return storageCredentials{}, false
	}
	if creds.CSIPublicKey == "" || creds.CHAPUsername == "" || creds.CHAPPassword == "" {
		return storageCredentials{}, false
	}
	return creds, true
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
	wf.Shell("Upload storage bundle to remote", c.stepUploadStorageBundle(s))
	wf.Shell("Install hostname and users", c.stepRunStorageInstall(s))
	wf.Shell("Provision ZFS pool and datasets", c.stepRunStoragePool(s))
	wf.Shell("Run storage health check", c.stepStorageHealth(s))
	wf.Shell("Harden default access", c.stepStorageHarden(s)).
		Unless(!c.hasOperatorKeys())
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
		s.vdevs = vdevs
		s.poolPlan, err = resolveStoragePoolPlan(c.Pool, s.inspection, vdevs)
		return err
	}
}

func (c *storageCmd) stepRenderStorageBundle(s *storageState) func(context.Context) error {
	return func(ctx context.Context) error {
		var err error
		s.bundle, err = buildStorageBundle(c.commonStorageFlags, c.ForceWipe, s.vdevs, s.csiPublicKey)
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
		SSHUser:                            "csi",
		CSIPrivateKey:                      s.csiPrivateKey,
		CSIPublicKey:                       s.csiPublicKey,
		CHAPUsername:                       s.chapUsername,
		CHAPPassword:                       s.chapPassword,
		ProvisionedAt:                      now,
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return "", storageSummary{}, err
	}
	path := filepath.Join(dir, "storage-appliance.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
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
	hardenStatus := "kairos password auth will be disabled"
	if !c.hasOperatorKeys() {
		hardenStatus = "SKIPPED — no operator keys supplied; kairos password auth stays enabled"
	}
	return []ui.KV{
		{Key: "Cluster target", Value: c.ClusterTarget},
		{Key: "Cluster name", Value: c.ClusterName},
		{Key: "SSH", Value: fmt.Sprintf("%s@%s:%d", c.SSHUser, c.Host, c.SSHPort)},
		{Key: "Image", Value: fmt.Sprintf("%s %s %s role=%s", s.metadata.Target, s.metadata.Arch, s.metadata.Hardware, s.metadata.Role)},
		{Key: "Node name", Value: c.NodeName},
		{Key: "Pool", Value: s.poolPlan.String()},
		{Key: "Pool options", Value: "ashift=12, autotrim=on, compatibility=" + c.PoolCompatibility},
		{Key: "Dataset parent", Value: c.datasetParent()},
		{Key: "Detached snapshots", Value: c.snapshotsParent()},
		{Key: "CSI user/key", Value: "csi, " + keyStatus},
		{Key: "CHAP", Value: chapStatus},
		{Key: "Hardening", Value: hardenStatus},
		{Key: "Reboot", Value: "not required"},
	}
}

// hasOperatorKeys reports whether any operator key flags were supplied.
// Hardening (disabling the default kairos password auth) without an
// operator key installed would lock the operator out of the node.
func (c *storageCmd) hasOperatorKeys() bool {
	return len(c.OperatorKey) > 0 || len(c.OperatorFiles) > 0
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

func (c *renderStorageCmd) Run(ctx *runContext) error {
	if c.ClusterName == "" {
		c.ClusterName = c.ClusterTarget
	}
	vdevs, err := parseStorageVDevs(c.PoolVDev, false)
	if err != nil {
		return err
	}
	if len(vdevs) == 0 {
		return fmt.Errorf("render storage requires --pool-vdev for the pool create path")
	}
	csiPublicKey, _, _, err := resolveCSIKey(c.CSIPublicKey)
	if err != nil {
		return err
	}
	bundle, err := buildStorageBundle(c.commonStorageFlags, false, vdevs, csiPublicKey)
	if err != nil {
		return err
	}
	if err := writeStorageBundle(c.OutputDir, bundle); err != nil {
		return err
	}
	successf("wrote storage bundle to %s", c.OutputDir)
	return nil
}

func buildStorageBundle(flags commonStorageFlags, forceWipe bool, vdevs []storageVDev, csiPublicKey string) (storageBundle, error) {
	operatorKeys, err := loadOptionalOperatorKeys(flags.OperatorKey, flags.OperatorFiles)
	if err != nil {
		return storageBundle{}, err
	}
	var authorizedKeys []byte
	var operatorActivation []byte
	if len(operatorKeys) > 0 {
		authorizedKeys = render.AuthorizedKeys(operatorKeys)
		operatorActivation = render.OperatorKeysActivationCloudConfig("K2 storage operator keys", "kairos", operatorKeys)
	}
	bundle := storageBundle{
		Activation:         render.HostnameActivationCloudConfig("K2 storage hostname activation", flags.NodeName),
		AuthorizedKeys:     authorizedKeys,
		OperatorActivation: operatorActivation,
		CSIPublicKey:       []byte(strings.TrimSpace(csiPublicKey) + "\n"),
		// Design D7: targetcli requires root; the csi key is treated as a root credential.
		CSISudoers: []byte("csi ALL=(ALL) NOPASSWD:ALL\n"),
	}
	bundle.InstallScript = []byte(storageInstallScript(flags.NodeName))
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
		"99-csi":                           bundle.CSISudoers,
		"storage-install.sh":               bundle.InstallScript,
		"storage-pool.sh":                  bundle.PoolScript,
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
