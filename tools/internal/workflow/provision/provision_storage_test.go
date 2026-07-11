package provision

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testOperatorKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeFakeFakeFakeFakeFakeFakeFakeFakeFakeFake operator"

func TestParseStorageVDevs(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		testVM  bool
		want    []string
		wantErr string
	}{
		{
			name:   "mirror by id",
			values: []string{"mirror ata-disk-a /dev/disk/by-id/ata-disk-b"},
			want:   []string{"mirror /dev/disk/by-id/ata-disk-a /dev/disk/by-id/ata-disk-b"},
		},
		{
			name:   "single device",
			values: []string{"ata-disk-a"},
			want:   []string{"/dev/disk/by-id/ata-disk-a"},
		},
		{
			name:    "plain dev rejected outside test vm",
			values:  []string{"/dev/sdb"},
			wantErr: "must be a /dev/disk/by-id path",
		},
		{
			name:   "plain dev allowed for test vm",
			values: []string{"/dev/sdb"},
			testVM: true,
			want:   []string{"/dev/sdb"},
		},
		{
			name:    "no topology means one device",
			values:  []string{"ata-a ata-b"},
			wantErr: "exactly one device",
		},
		{
			name:    "duplicate devices rejected",
			values:  []string{"ata-a", "/dev/disk/by-id/ata-a"},
			wantErr: "duplicate pool device",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStorageVDevs(tt.values, tt.testVM)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var gotStrings []string
			for _, vdev := range got {
				gotStrings = append(gotStrings, vdev.String())
			}
			if strings.Join(gotStrings, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("vdevs = %#v, want %#v", gotStrings, tt.want)
			}
		})
	}
}

func TestParseStorageDisksClassifiesAndExcludesBootDisk(t *testing.T) {
	lsblk := []byte(`{
  "blockdevices": [
    {"name":"sda","type":"disk","size":10737418240,"model":"Boot","children":[
      {"name":"sda1","type":"part","size":1,"label":"COS_STATE","mountpoint":"/run/initramfs/cos-state"}
    ]},
    {"name":"sdb","type":"disk","size":1000000000,"model":"Blank"},
    {"name":"sdc","type":"disk","size":2000000000,"model":"Parted","children":[
      {"name":"sdc1","type":"part","size":1,"fstype":"ext4"}
    ]},
    {"name":"sdd","type":"disk","size":3000000000,"model":"ZFS","fstype":"zfs_member","label":"tank"},
    {"name":"sde","type":"disk","size":4000000000,"model":"Mounted","mountpoint":"/mnt/data"}
  ]
}`)
	byID := []byte(`
lrwxrwxrwx 1 root root 9 Jul  8 00:00 ata-Blank -> ../../sdb
lrwxrwxrwx 1 root root 9 Jul  8 00:00 ata-Parted -> ../../sdc
lrwxrwxrwx 1 root root 9 Jul  8 00:00 ata-ZFS -> ../../sdd
lrwxrwxrwx 1 root root 9 Jul  8 00:00 ata-Mounted -> ../../sde
lrwxrwxrwx 1 root root 9 Jul  8 00:00 ata-Boot -> ../../sda
`)
	got, err := parseStorageDisks(lsblk, byID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("disk count = %d, want 4: %#v", len(got), got)
	}
	states := map[string]storageDiskState{}
	for _, disk := range got {
		states[disk.ByID] = disk.State
		if strings.Contains(disk.ByID, "Boot") {
			t.Fatalf("boot disk was listed: %#v", disk)
		}
	}
	want := map[string]storageDiskState{
		"/dev/disk/by-id/ata-Blank":   storageDiskBlank,
		"/dev/disk/by-id/ata-Parted":  "partitioned(ext4)",
		"/dev/disk/by-id/ata-ZFS":     "zfs-member(tank)",
		"/dev/disk/by-id/ata-Mounted": storageDiskMounted,
	}
	for disk, state := range want {
		if states[disk] != state {
			t.Fatalf("%s state = %q, want %q", disk, states[disk], state)
		}
	}
}

func TestResolveStoragePoolPlan(t *testing.T) {
	vdevs := []storageVDev{{Devices: []string{"/dev/disk/by-id/ata-a"}}}
	tests := []struct {
		name       string
		inspection storageInspection
		vdevs      []storageVDev
		want       storagePoolVerdict
		wantErr    string
	}{
		{name: "create", inspection: storageInspection{PoolState: storagePoolMissing}, vdevs: vdevs, want: storagePoolCreate},
		{name: "import", inspection: storageInspection{PoolState: storagePoolImportable}, want: storagePoolImport},
		{name: "already imported", inspection: storageInspection{PoolState: storagePoolImported, PoolHealth: "ONLINE"}, want: storagePoolAlreadyImported},
		// vdevs alongside an existing pool are tolerated (the caller drops
		// them with a note) so scenario reruns stay plain reruns.
		{name: "vdevs with existing pool", inspection: storageInspection{PoolState: storagePoolImported, PoolHealth: "ONLINE"}, vdevs: vdevs, want: storagePoolAlreadyImported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveStoragePoolPlan("tank", tt.inspection, tt.vdevs)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q, want %q", got.Verdict, tt.want)
			}
		})
	}
}

func TestStorageKeyCHAPAndCredentialSecretBoundaries(t *testing.T) {
	pub, priv, generated, err := resolveCSIKey("")
	if err != nil {
		t.Fatal(err)
	}
	if !generated || !strings.HasPrefix(pub, "ssh-ed25519 ") || !strings.Contains(priv, "OPENSSH PRIVATE KEY") {
		t.Fatalf("unexpected generated keypair: generated=%v pub=%q priv=%q", generated, pub, priv[:min(len(priv), 32)])
	}
	chap, err := randomBase62(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(chap) != 16 || strings.ContainsFunc(chap, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", r)
	}) {
		t.Fatalf("invalid chap secret %q", chap)
	}
	poolKey, err := generatePoolKey()
	if err != nil {
		t.Fatal(err)
	}
	rawPoolKey, err := decodePoolKey(poolKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(rawPoolKey) != 32 {
		t.Fatalf("raw pool key length = %d, want 32", len(rawPoolKey))
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := storageCmd{
		commonStorageFlags: commonStorageFlags{ClusterName: "v3", Pool: "tank", IQNBase: "iqn.test"},
		Host:               "10.0.0.2",
		SSHPort:            22,
		Portal:             "10.0.0.2:3260",
	}
	state := &storageState{csiPublicKey: pub, csiPrivateKey: priv, chapUsername: "k2-v3", chapPassword: chap, poolKey: poolKey}
	path, summary, err := cmd.writeStorageCredentials(state)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "csiPrivateKey") || !strings.Contains(string(data), "chapPassword") {
		t.Fatalf("credential file missing secrets:\n%s", string(data))
	}
	if !strings.Contains(string(data), "poolKey") || !strings.Contains(string(data), poolKey) {
		t.Fatalf("credential file missing pool key:\n%s", string(data))
	}
	summaryData, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(summaryData, []byte("csiPrivateKey")) ||
		bytes.Contains(summaryData, []byte("chapPassword")) ||
		bytes.Contains(summaryData, []byte("poolKey")) ||
		bytes.Contains(summaryData, []byte(chap)) ||
		bytes.Contains(summaryData, []byte(poolKey)) {
		t.Fatalf("summary leaked secret fields: %s", string(summaryData))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadStorageCredentialsFixedSchema(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".kube", "k2", "v3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := `{
  "portal": "10.0.0.2:3260",
  "iqnBase": "iqn.test",
  "pool": "tank",
  "datasetParentName": "tank/csi/v3",
  "detachedSnapshotsDatasetParentName": "tank/csi/v3-snapshots",
  "sshHost": "10.0.0.2",
  "sshPort": 22,
  "sshUser": "csi",
  "csiPrivateKey": "PRIVATE",
  "csiPublicKey": "ssh-ed25519 PUBLIC",
  "chapUsername": "k2-v3",
  "chapPassword": "CHAP",
  "provisionedAt": "2026-07-08T00:00:00Z"
}
`
	if err := os.WriteFile(filepath.Join(dir, "storage-appliance.json"), []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok, err := loadStorageCredentials("v3")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("loadStorageCredentials ok = false, want true")
	}
	if got.Portal != "10.0.0.2:3260" ||
		got.IQNBase != "iqn.test" ||
		got.Pool != "tank" ||
		got.DatasetParentName != "tank/csi/v3" ||
		got.DetachedSnapshotsDatasetParentName != "tank/csi/v3-snapshots" ||
		got.SSHHost != "10.0.0.2" ||
		got.SSHPort != 22 ||
		got.SSHUser != "csi" ||
		got.CSIPrivateKey != "PRIVATE" ||
		got.CSIPublicKey != "ssh-ed25519 PUBLIC" ||
		got.CHAPUsername != "k2-v3" ||
		got.CHAPPassword != "CHAP" ||
		got.PoolKey != "" ||
		got.ProvisionedAt != "2026-07-08T00:00:00Z" {
		t.Fatalf("credentials = %#v", got)
	}
}

func TestWriteLoadStorageCredentialsRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := storageCmd{
		commonStorageFlags: commonStorageFlags{ClusterName: "v3", Pool: "tank", IQNBase: "iqn.test"},
		Host:               "10.0.0.2",
		SSHPort:            2222,
		Portal:             "10.0.0.2:3260",
	}
	poolKey, err := generatePoolKey()
	if err != nil {
		t.Fatal(err)
	}
	state := &storageState{
		csiPublicKey:  "ssh-ed25519 PUBLIC",
		csiPrivateKey: "PRIVATE",
		chapUsername:  "k2-v3",
		chapPassword:  "CHAP",
		poolKey:       poolKey,
	}

	if _, _, err := cmd.writeStorageCredentials(state); err != nil {
		t.Fatal(err)
	}
	got, ok, err := loadStorageCredentials("v3")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("loadStorageCredentials ok = false, want true")
	}
	if got.CSIPrivateKey != state.csiPrivateKey ||
		got.CSIPublicKey != state.csiPublicKey ||
		got.CHAPUsername != state.chapUsername ||
		got.CHAPPassword != state.chapPassword ||
		got.PoolKey != state.poolKey {
		t.Fatalf("loaded credentials = %#v", got)
	}
}

func TestNewStorageStateBypassesCredentialReuseWhenRotatingOrKeySupplied(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := storageCmd{
		commonStorageFlags: commonStorageFlags{ClusterName: "v3"},
		Host:               "10.0.0.2",
		SSHPort:            22,
	}
	poolKey, err := generatePoolKey()
	if err != nil {
		t.Fatal(err)
	}
	old := storageCredentials{CSIPublicKey: "old-public", CSIPrivateKey: "old-private", CHAPUsername: "old-user", CHAPPassword: "old-pass", PoolKey: poolKey}
	dir := filepath.Join(home, ".kube", "k2", "v3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "storage-appliance.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd.RotateCredentials = true
	rotated, err := cmd.newStorageState()
	if err != nil {
		t.Fatal(err)
	}
	if rotated.csiPublicKey == old.CSIPublicKey || rotated.chapPassword == old.CHAPPassword {
		t.Fatalf("rotated state reused old credentials: %#v", rotated)
	}
	if rotated.poolKey != old.PoolKey {
		t.Fatalf("rotated pool key = %q, want preserved %q", rotated.poolKey, old.PoolKey)
	}

	supplied, _, _, err := resolveCSIKey("")
	if err != nil {
		t.Fatal(err)
	}
	cmd.RotateCredentials = false
	cmd.CSIPublicKey = supplied
	withKey, err := cmd.newStorageState()
	if err != nil {
		t.Fatal(err)
	}
	if withKey.csiPublicKey != supplied || withKey.csiPrivateKey != "" || withKey.chapPassword == old.CHAPPassword || withKey.poolKey != old.PoolKey {
		t.Fatalf("supplied-key state = %#v", withKey)
	}
}

func TestNewStorageStateReusesOldCredentialsAndAddsMissingPoolKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := storageCmd{
		commonStorageFlags: commonStorageFlags{ClusterName: "v3"},
		Host:               "10.0.0.2",
		SSHPort:            22,
	}
	old := storageCredentials{CSIPublicKey: "old-public", CSIPrivateKey: "old-private", CHAPUsername: "old-user", CHAPPassword: "old-pass"}
	dir := filepath.Join(home, ".kube", "k2", "v3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "storage-appliance.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := cmd.newStorageState()
	if err != nil {
		t.Fatal(err)
	}
	if got.csiPublicKey != old.CSIPublicKey ||
		got.csiPrivateKey != old.CSIPrivateKey ||
		got.chapUsername != old.CHAPUsername ||
		got.chapPassword != old.CHAPPassword {
		t.Fatalf("state did not reuse existing fields: %#v", got)
	}
	if _, err := decodePoolKey(got.poolKey); err != nil {
		t.Fatalf("generated pool key invalid: %v", err)
	}
}

func TestLoadStorageCredentialsMissingCHAPPasswordErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".kube", "k2", "v3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "storage-appliance.json"), []byte(`{"csiPublicKey":"ssh-ed25519 PUBLIC","chapUsername":"k2-v3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// An existing-but-incomplete file must ERROR, not read as "absent":
	// silently regenerating credentials also rotates the pool wrapping key
	// of an existing encrypted pool.
	if _, ok, err := loadStorageCredentials("v3"); err == nil || ok {
		t.Fatalf("loadStorageCredentials = ok %v err %v, want error", ok, err)
	}
}

func TestStorageStateFromCredentialsCopiesSecrets(t *testing.T) {
	creds := storageCredentials{
		CSIPrivateKey: "PRIVATE",
		CSIPublicKey:  "PUBLIC",
		CHAPUsername:  "USER",
		CHAPPassword:  "PASS",
		PoolKey:       "POOL",
	}
	got := (&storageCmd{}).storageStateFromCredentials(creds)
	if got.csiPrivateKey != creds.CSIPrivateKey ||
		got.csiPublicKey != creds.CSIPublicKey ||
		got.chapUsername != creds.CHAPUsername ||
		got.chapPassword != creds.CHAPPassword ||
		got.poolKey != creds.PoolKey {
		t.Fatalf("state = %#v", got)
	}
}

func TestRandomBase62LengthAndCharset(t *testing.T) {
	got, err := randomBase62(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 16 {
		t.Fatalf("length = %d, want 16", len(got))
	}
	if strings.ContainsFunc(got, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", r)
	}) {
		t.Fatalf("randomBase62 used non-base62 character: %q", got)
	}
}

func TestPromptStorageVDevs(t *testing.T) {
	disks := []storageDisk{
		{ByID: "/dev/disk/by-id/ata-a", State: storageDiskBlank, Size: 1, Model: "A"},
		{ByID: "/dev/disk/by-id/ata-b", State: storageDiskBlank, Size: 1, Model: "B"},
	}
	got, err := promptStorageVDevs(strings.NewReader("mirror 1 2\n\n"), ioDiscard{}, disks, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].String() != "mirror /dev/disk/by-id/ata-a /dev/disk/by-id/ata-b" {
		t.Fatalf("vdevs = %#v", got)
	}

	disks[0].State = "partitioned(ext4)"
	_, err = promptStorageVDevs(strings.NewReader("1\n"), ioDiscard{}, disks, false)
	if err == nil || !strings.Contains(err.Error(), "--force-wipe") {
		t.Fatalf("error = %v, want force-wipe rejection", err)
	}
}

func TestPromptStorageVDevsRejectsDuplicateSelectionsWithReprompt(t *testing.T) {
	disks := []storageDisk{
		{ByID: "/dev/disk/by-id/ata-a", State: storageDiskBlank, Size: 1, Model: "A"},
		{ByID: "/dev/disk/by-id/ata-b", State: storageDiskBlank, Size: 1, Model: "B"},
	}
	var out bytes.Buffer
	got, err := promptStorageVDevs(strings.NewReader("mirror 1 1\nmirror 1 2\n\n"), &out, disks, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].String() != "mirror /dev/disk/by-id/ata-a /dev/disk/by-id/ata-b" {
		t.Fatalf("vdevs = %#v", got)
	}
	if !strings.Contains(out.String(), "duplicate disk selection") {
		t.Fatalf("prompt output missing duplicate warning:\n%s", out.String())
	}

	out.Reset()
	got, err = promptStorageVDevs(strings.NewReader("1\n1\n2\n\n"), &out, disks, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].String() != "/dev/disk/by-id/ata-a" || got[1].String() != "/dev/disk/by-id/ata-b" {
		t.Fatalf("vdevs = %#v", got)
	}
}

func TestBuildStorageBundleGoldenStyle(t *testing.T) {
	pub, _, _, err := resolveCSIKey("")
	if err != nil {
		t.Fatal(err)
	}
	poolKey, err := generatePoolKey()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := buildStorageBundle(commonStorageFlags{
		ClusterName:       "v3",
		NodeName:          "k2-storage",
		Pool:              "tank",
		PoolCompatibility: "openzfs-2.2-linux",
		OperatorKey:       []string{testOperatorKey},
	}, false, []storageVDev{{Topology: "mirror", Devices: []string{"/dev/disk/by-id/ata-a", "/dev/disk/by-id/ata-b"}}}, pub, poolKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := writeStorageBundle(dir, bundle); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"99-k2-storage-hostname.yaml": "hostname: k2-storage",
		"99-csi":                      "csi ALL=(ALL) NOPASSWD:ALL",
		// Reboot persistence: the csi user must be stage-managed, and the
		// install script must place the stage in /oem (asserted below).
		"95-k2-storage-csi.yaml": "/etc/sudoers.d/99-csi",
		"storage-install.sh":     "hostnamectl set-hostname 'k2-storage'",
		"storage-pool.sh":        "sudo zpool create -m none",
		"zfs_pool.key":           string(bundle.PoolKey),
	}
	for file, want := range checks {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s missing %q:\n%s", file, want, string(data))
		}
	}
	install, err := os.ReadFile(filepath.Join(dir, "storage-install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(install), `sudo install -m 0644 "$remote_dir"/95-k2-storage-csi.yaml /oem/95-k2-storage-csi.yaml`) {
		t.Fatalf("install script does not stage the csi user activation into /oem:\n%s", install)
	}
	info, err := os.Stat(filepath.Join(dir, "zfs_pool.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("zfs_pool.key mode = %v, want 0600", info.Mode().Perm())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
