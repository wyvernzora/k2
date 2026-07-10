package toolcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

type storageVDev struct {
	Topology string
	Devices  []string
}

func (v storageVDev) String() string {
	if v.Topology == "" {
		return strings.Join(v.Devices, " ")
	}
	return v.Topology + " " + strings.Join(v.Devices, " ")
}

type storagePoolState string

const (
	storagePoolMissing    storagePoolState = "missing"
	storagePoolImportable storagePoolState = "importable"
	storagePoolImported   storagePoolState = "imported"
)

type storagePoolVerdict string

const (
	storagePoolCreate          storagePoolVerdict = "CREATE"
	storagePoolImport          storagePoolVerdict = "IMPORT existing"
	storagePoolAlreadyImported storagePoolVerdict = "ALREADY IMPORTED"
)

type storagePoolPlan struct {
	Pool    string
	Verdict storagePoolVerdict
	Health  string
	VDevs   []storageVDev
}

func (p storagePoolPlan) String() string {
	switch p.Verdict {
	case storagePoolCreate:
		parts := make([]string, len(p.VDevs))
		for i, vdev := range p.VDevs {
			parts[i] = vdev.String()
		}
		return fmt.Sprintf("%s %s (%s)", p.Pool, p.Verdict, strings.Join(parts, "; "))
	case storagePoolAlreadyImported:
		if p.Health != "" {
			return fmt.Sprintf("%s %s (%s)", p.Pool, p.Verdict, p.Health)
		}
	}
	return fmt.Sprintf("%s %s", p.Pool, p.Verdict)
}

type storageInspection struct {
	PoolState  storagePoolState
	PoolHealth string
	Disks      []storageDisk
}

type storageDisk struct {
	ByID   string
	Kernel string
	Size   int64
	Model  string
	State  storageDiskState
}

type storageDiskState string

const (
	storageDiskBlank       storageDiskState = "blank"
	storageDiskPartitioned storageDiskState = "partitioned"
	storageDiskZFSMember   storageDiskState = "zfs-member"
	storageDiskMounted     storageDiskState = "mounted"
)

func (s storageDiskState) selectable(force bool) bool {
	return s == storageDiskBlank || force
}

func parseStorageVDevs(values []string, testVM bool) ([]storageVDev, error) {
	var out []storageVDev
	seen := map[string]bool{}
	for _, value := range values {
		vdev, err := parseStorageVDev(value, testVM)
		if err != nil {
			return nil, err
		}
		for _, dev := range vdev.Devices {
			if seen[dev] {
				return nil, fmt.Errorf("duplicate pool device %s", dev)
			}
			seen[dev] = true
		}
		out = append(out, vdev)
	}
	return out, nil
}

func parseStorageVDev(value string, testVM bool) (storageVDev, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return storageVDev{}, fmt.Errorf("empty --pool-vdev")
	}
	topology, fields, err := parseStorageVDevShape(fields)
	if err != nil {
		return storageVDev{}, fmt.Errorf("pool vdev %q %w", value, err)
	}
	devices := make([]string, len(fields))
	for i, field := range fields {
		dev, err := normalizeStorageDevice(field, testVM)
		if err != nil {
			return storageVDev{}, err
		}
		devices[i] = dev
	}
	return storageVDev{Topology: topology, Devices: devices}, nil
}

func parseStorageVDevShape(fields []string) (topology string, devices []string, err error) {
	if len(fields) > 0 && isStorageTopology(fields[0]) {
		topology = fields[0]
		fields = fields[1:]
	}
	if topology == "" && len(fields) != 1 {
		return "", nil, fmt.Errorf("has no topology keyword; pass exactly one device or add mirror/raidz/raidz2/raidz3")
	}
	if topology != "" && len(fields) < 2 {
		return "", nil, fmt.Errorf("%s vdev requires at least two devices", topology)
	}
	return topology, fields, nil
}

func isStorageTopology(value string) bool {
	switch value {
	case "mirror", "raidz", "raidz2", "raidz3":
		return true
	default:
		return false
	}
}

func normalizeStorageDevice(value string, testVM bool) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return "", fmt.Errorf("empty device path")
	case strings.HasPrefix(value, "/dev/disk/by-id/"):
		return value, nil
	case !strings.HasPrefix(value, "/dev/"):
		return "/dev/disk/by-id/" + value, nil
	case testVM:
		return value, nil
	default:
		return "", fmt.Errorf("pool device %s must be a /dev/disk/by-id path", value)
	}
}

func resolveStoragePoolPlan(pool string, inspection storageInspection, vdevs []storageVDev) (storagePoolPlan, error) {
	switch inspection.PoolState {
	case storagePoolImported:
		return storagePoolPlan{Pool: pool, Verdict: storagePoolAlreadyImported, Health: inspection.PoolHealth}, nil
	case storagePoolImportable:
		return storagePoolPlan{Pool: pool, Verdict: storagePoolImport}, nil
	default:
		if len(vdevs) == 0 {
			return storagePoolPlan{}, fmt.Errorf("no pool present; pass --pool-vdev or run interactively")
		}
		return storagePoolPlan{Pool: pool, Verdict: storagePoolCreate, VDevs: vdevs}, nil
	}
}

func promptStorageVDevs(r io.Reader, w io.Writer, disks []storageDisk, force bool) ([]storageVDev, error) {
	if len(disks) == 0 {
		return nil, fmt.Errorf("no candidate disks found")
	}
	for i, disk := range disks {
		fmt.Fprintf(w, "%2d  %-48s %-8s %-24s %s\n", i+1, strings.TrimPrefix(disk.ByID, "/dev/disk/by-id/"), humanBytes(disk.Size), disk.Model, disk.State)
	}
	scanner := bufio.NewScanner(r)
	var out []storageVDev
	seen := map[string]bool{}
	for n := 1; ; n++ {
		fmt.Fprintf(w, "vdev %d -- topology+disk #s (e.g. \"mirror 1 2\"), empty line to finish: ", n)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, err
			}
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}
		vdev, err := parseStoragePickerLine(line, disks, force)
		if err != nil {
			return nil, err
		}
		duplicate := ""
		local := map[string]bool{}
		for _, dev := range vdev.Devices {
			if local[dev] || seen[dev] {
				duplicate = dev
				break
			}
			local[dev] = true
		}
		if duplicate != "" {
			fmt.Fprintf(w, "duplicate disk selection %s\n", strings.TrimPrefix(duplicate, "/dev/disk/by-id/"))
			n--
			continue
		}
		for _, dev := range vdev.Devices {
			seen[dev] = true
		}
		out = append(out, vdev)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no vdevs selected")
	}
	return out, nil
}

func parseStoragePickerLine(line string, disks []storageDisk, force bool) (storageVDev, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return storageVDev{}, fmt.Errorf("empty vdev selection")
	}
	topology, fields, err := parseStorageVDevShape(fields)
	if err != nil {
		return storageVDev{}, err
	}
	devices := make([]string, len(fields))
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil || n < 1 || n > len(disks) {
			return storageVDev{}, fmt.Errorf("invalid disk selection %q", field)
		}
		disk := disks[n-1]
		if !disk.State.selectable(force) {
			return storageVDev{}, fmt.Errorf("disk %d is %s; pass --force-wipe to select non-blank disks", n, disk.State)
		}
		devices[i] = disk.ByID
	}
	return storageVDev{Topology: topology, Devices: devices}, nil
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Size       json.RawMessage `json:"size"`
	Model      string          `json:"model"`
	FSType     string          `json:"fstype"`
	Label      string          `json:"label"`
	Mountpoint string          `json:"mountpoint"`
	Children   []lsblkDevice   `json:"children"`
}

func parseStorageDisks(lsblkJSON []byte, byIDListing []byte) ([]storageDisk, error) {
	var decoded lsblkOutput
	if err := json.Unmarshal(lsblkJSON, &decoded); err != nil {
		return nil, fmt.Errorf("parse lsblk JSON: %w", err)
	}
	aliases := parseByIDListing(byIDListing)
	var out []storageDisk
	for _, dev := range decoded.BlockDevices {
		if dev.Type != "disk" {
			continue
		}
		if isBootDisk(dev) {
			continue
		}
		byID := aliases[dev.Name]
		if byID == "" {
			continue
		}
		out = append(out, storageDisk{
			ByID:   "/dev/disk/by-id/" + byID,
			Kernel: dev.Name,
			Size:   rawJSONInt64(dev.Size),
			Model:  strings.TrimSpace(dev.Model),
			State:  classifyDisk(dev),
		})
	}
	return out, nil
}

func parseByIDListing(data []byte) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, " -> ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[len(fields)-3]
		target := filepath.Base(fields[len(fields)-1])
		if strings.Contains(name, "-part") || strings.HasPrefix(name, "wwn-") || strings.HasPrefix(name, "eui.") {
			continue
		}
		if _, exists := out[target]; !exists {
			out[target] = name
		}
	}
	return out
}

func rawJSONInt64(data json.RawMessage) int64 {
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		n, _ = strconv.ParseInt(s, 10, 64)
	}
	return n
}

func classifyDisk(dev lsblkDevice) storageDiskState {
	if hasMountpoint(dev) {
		return storageDiskMounted
	}
	if pool := zfsPoolLabel(dev); pool != "" {
		return storageDiskState("zfs-member(" + pool + ")")
	}
	if hasPartitionOrSignature(dev) {
		return storageDiskState(partitionedState(dev))
	}
	return storageDiskBlank
}

func hasMountpoint(dev lsblkDevice) bool {
	if strings.TrimSpace(dev.Mountpoint) != "" {
		return true
	}
	for _, child := range dev.Children {
		if hasMountpoint(child) {
			return true
		}
	}
	return false
}

func zfsPoolLabel(dev lsblkDevice) string {
	if strings.EqualFold(dev.FSType, "zfs_member") || strings.EqualFold(dev.FSType, "zfs-member") {
		if dev.Label != "" {
			return dev.Label
		}
		return "unknown"
	}
	for _, child := range dev.Children {
		if pool := zfsPoolLabel(child); pool != "" {
			return pool
		}
	}
	return ""
}

func hasPartitionOrSignature(dev lsblkDevice) bool {
	if dev.FSType != "" || dev.Label != "" || len(dev.Children) > 0 {
		return true
	}
	return false
}

func partitionedState(dev lsblkDevice) string {
	fstype := firstFSType(dev)
	if fstype == "" {
		return string(storageDiskPartitioned)
	}
	return "partitioned(" + fstype + ")"
}

func firstFSType(dev lsblkDevice) string {
	if dev.FSType != "" {
		return dev.FSType
	}
	for _, child := range dev.Children {
		if fstype := firstFSType(child); fstype != "" {
			return fstype
		}
	}
	return ""
}

func isBootDisk(dev lsblkDevice) bool {
	return hasCOSLabel(dev) || hasCOSStateMount(dev)
}

func hasCOSLabel(dev lsblkDevice) bool {
	if dev.Label == "COS_STATE" || dev.Label == "COS_GRUB" {
		return true
	}
	for _, child := range dev.Children {
		if hasCOSLabel(child) {
			return true
		}
	}
	return false
}

func hasCOSStateMount(dev lsblkDevice) bool {
	if dev.Mountpoint == "/run/initramfs/cos-state" {
		return true
	}
	for _, child := range dev.Children {
		if hasCOSStateMount(child) {
			return true
		}
	}
	return false
}
