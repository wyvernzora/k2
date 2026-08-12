package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wyvernzora/k2/kairos/node-agent/internal/buildmetadata"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/health"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/runner"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/textfile"
)

const (
	// DefaultTextfileDir is the Ubuntu prometheus-node-exporter package's
	// textfile collector directory; node_exporter serves anything written
	// here on :9100 and exposes node_textfile_mtime_seconds for staleness.
	DefaultTextfileDir = "/var/lib/prometheus/node-exporter"
	promFileName       = "k2.prom"

	defaultConfigFSRoot = "/sys/kernel/config/target/iscsi"
	defaultSaveConfig   = "/etc/rtslib-fb-target/saveconfig.json"

	// GRUB's boot-assessment sentinels live in this env block on COS_STATE,
	// and /run/cos holds the markers immucore writes for the current boot.
	defaultBootAssessment = "/run/initramfs/cos-state/boot_assessment"
	defaultRunCosDir      = "/run/cos"
)

type Config struct {
	TextfileDir        string
	ConfigFSRoot       string
	SaveConfig         string
	StatusFile         string
	MetadataFile       string
	BootAssessmentFile string
	RunCosDir          string
	Debug              io.Writer
}

// desc identifies a metric family; help is rendered once per family.
type desc struct {
	name   string
	help   string
	labels []string
}

type Collector struct {
	cfg Config
	run runner.Runner

	collectorSuccess *desc
	nodeBuildInfo    *desc
	zfsPoolHealth    *desc
	zfsPoolSize      *desc
	zfsPoolAlloc     *desc
	zfsPoolFrag      *desc
	zfsPoolCap       *desc
	zfsKeyStatus     *desc
	zfsVolumeSize    *desc
	zfsVolumeUsed    *desc
	zfsVolumes       *desc
	lioTargets       *desc
	lioLUNs          *desc
	lioSessions      *desc
	lioSaveInSync    *desc
	smartTemp        *desc
	smartPctUsed     *desc
	smartMediaErrors *desc
	smartPowerHours  *desc
	storageHealthy   *desc
	storageLastRun   *desc
	snapshotLast     *desc
	snapshotCount    *desc
	bootAssessArmed  *desc
	bootAssessOn     *desc
	bootPassive      *desc
	bootUpgradeFail  *desc
}

type groupResult struct {
	samples []sample
	success bool
}

type sample struct {
	desc   *desc
	value  float64
	labels []string
}

// Run collects every applicable group once, renders Prometheus text exposition, and
// atomically replaces <textfile-dir>/k2.prom. Designed to run as a systemd
// oneshot on a timer; freshness is monitored via node_textfile_mtime_seconds.
func Run(cfg Config) error {
	cfg = normalize(cfg)
	c := NewCollector(cfg, runner.OSRunner{})
	return writeTextfile(filepath.Join(cfg.TextfileDir, promFileName), c.Render())
}

func NewCollector(cfg Config, run runner.Runner) *Collector {
	cfg = normalize(cfg)
	return &Collector{
		cfg: cfg,
		run: run,

		collectorSuccess: &desc{"k2_collector_success", "Whether the K2 metrics collector group succeeded.", []string{"collector"}},
		nodeBuildInfo: &desc{
			"k2_node_build_info",
			"K2 node image build information.",
			[]string{"target", "flavor", "flavor_version", "kairos_version", "kairos_agent_version", "kubernetes_distro", "kubernetes_version", "role", "arch", "hardware", "source_commit"},
		},
		zfsPoolHealth:    &desc{"k2_zfs_pool_health", "ZFS pool health, 1 when ONLINE.", []string{"pool"}},
		zfsPoolSize:      &desc{"k2_zfs_pool_size_bytes", "ZFS pool size in bytes.", []string{"pool"}},
		zfsPoolAlloc:     &desc{"k2_zfs_pool_alloc_bytes", "ZFS pool allocated bytes.", []string{"pool"}},
		zfsPoolFrag:      &desc{"k2_zfs_pool_fragmentation_ratio", "ZFS pool fragmentation ratio.", []string{"pool"}},
		zfsPoolCap:       &desc{"k2_zfs_pool_capacity_ratio", "ZFS pool capacity ratio.", []string{"pool"}},
		zfsKeyStatus:     &desc{"k2_zfs_keystatus_available", "ZFS encrypted dataset key availability.", []string{"dataset"}},
		zfsVolumeSize:    &desc{"k2_zfs_volume_size_bytes", "ZFS volume size in bytes.", []string{"volume"}},
		zfsVolumeUsed:    &desc{"k2_zfs_volume_used_bytes", "ZFS volume used bytes.", []string{"volume"}},
		zfsVolumes:       &desc{"k2_zfs_volumes", "Total ZFS volume count.", nil},
		lioTargets:       &desc{"k2_lio_targets", "LIO iSCSI target count.", nil},
		lioLUNs:          &desc{"k2_lio_luns", "LIO LUN count.", nil},
		lioSessions:      &desc{"k2_lio_sessions", "LIO node ACL count; ACLs are cached, so this is not a live session count.", nil},
		lioSaveInSync:    &desc{"k2_lio_saveconfig_in_sync", "Whether live LIO target count matches saveconfig.", nil},
		smartTemp:        &desc{"k2_smart_temperature_celsius", "SMART temperature in Celsius.", []string{"device"}},
		smartPctUsed:     &desc{"k2_smart_percentage_used", "NVMe SMART percentage used.", []string{"device"}},
		smartMediaErrors: &desc{"k2_smart_media_errors", "SMART media error count.", []string{"device"}},
		smartPowerHours:  &desc{"k2_smart_power_on_hours", "SMART power-on hours.", []string{"device"}},
		storageHealthy:   &desc{"k2_storage_healthy", "K2 storage health status.", nil},
		storageLastRun:   &desc{"k2_storage_health_last_run_timestamp_seconds", "Unix timestamp of the last storage health status write.", nil},
		snapshotLast:     &desc{"k2_zfs_last_snapshot_timestamp_seconds", "Unix creation time of the newest cadence snapshot per prefix.", []string{"prefix"}},
		snapshotCount:    &desc{"k2_zfs_snapshot_count", "Distinct cadence snapshot points retained per prefix.", []string{"prefix"}},
		bootAssessArmed:  &desc{"k2_boot_assessment_armed", "Whether GRUB's boot_assessment_tentative sentinel is still set; the next reboot would roll this node back.", nil},
		bootAssessOn:     &desc{"k2_boot_assessment_enabled", "Whether GRUB boot assessment is enabled for this node.", nil},
		bootPassive:      &desc{"k2_boot_slot_passive", "Whether this boot came up on the passive (fallback) slot.", nil},
		bootUpgradeFail:  &desc{"k2_boot_upgrade_failure", "Whether this boot was stamped as following a failed upgrade.", nil},
	}
}

// Render collects all groups and returns the exposition text.
func (c *Collector) Render() string {
	type collectorGroup struct {
		name string
		fn   func() groupResult
	}
	// boot_assessment applies to every Kairos node regardless of role.
	groups := []collectorGroup{
		{"node_build", c.collectNodeBuild},
		{"boot_assessment", c.collectBootAssessment},
	}
	metadata, err := readBuildMetadata(c.cfg.MetadataFile)
	if err == nil && metadata["role"] == "storage" {
		groups = append(groups,
			collectorGroup{"zfs_pools", c.collectZFSPools},
			collectorGroup{"zfs_keystatus", c.collectZFSKeyStatus},
			collectorGroup{"zfs_volumes", c.collectZFSVolumes},
			collectorGroup{"zfs_snapshots", c.collectZFSSnapshots},
			collectorGroup{"lio", c.collectLIO},
			collectorGroup{"smart", c.collectSMART},
			collectorGroup{"storage_health", c.collectStorageHealth},
		)
	}

	var samples []sample
	for _, group := range groups {
		result := group.fn()
		samples = append(samples, result.samples...)
		success := 0.0
		if result.success {
			success = 1
		}
		samples = append(samples, sample{desc: c.collectorSuccess, value: success, labels: []string{group.name}})
	}
	return renderExposition(samples)
}

func (c *Collector) collectNodeBuild() groupResult {
	metadata, err := readBuildMetadata(c.cfg.MetadataFile)
	if err != nil {
		c.debugf("read node build metadata failed: %v", err)
		return groupResult{success: false}
	}
	if metadata["target"] == "" {
		c.debugf("node build metadata has empty target")
		return groupResult{success: false}
	}

	flavor, flavorVersion := splitFlavor(metadata["flavor"])
	kairosAgentVersion := ""
	// Node identity remains useful on valid images where kairos-agent is absent
	// or cannot report its version, so this subprocess is deliberately optional.
	if out, err := c.run.Output("kairos-agent", "--version"); err == nil {
		fields := strings.Fields(out)
		if len(fields) > 0 {
			kairosAgentVersion = fields[len(fields)-1]
		}
	}

	return groupResult{
		success: true,
		samples: []sample{{
			desc:  c.nodeBuildInfo,
			value: 1,
			labels: []string{
				metadata["target"],
				flavor,
				flavorVersion,
				metadata["kairosVersion"],
				kairosAgentVersion,
				metadata["kubernetesDistro"],
				metadata["kubernetesVersion"],
				metadata["role"],
				metadata["arch"],
				metadata["hardware"],
				metadata["sourceCommit"],
			},
		}},
	}
}

// collectBootAssessment reports whether this node is carrying a live GRUB
// boot-assessment sentinel, and whether it already fell back.
//
// Written after k2-pi-35d9 rolled back on 2026-08-12. Kairos sets
// boot_assessment_tentative=yes on the first boot after an upgrade and clears
// it once that boot succeeds; if the clearing never happens the flag sits
// there indefinitely, and the NEXT reboot — whenever that is, in that case 30
// days later during unrelated rack work — makes GRUB conclude the last boot
// failed and boot the passive slot instead. The node looks perfectly healthy
// the whole time it is armed, which is exactly what made this expensive to
// find: a control-plane node silently reverted to a months-old image.
//
// k2_boot_assessment_armed is the predictive signal (alert while it can still
// be cleared); k2_boot_slot_passive and k2_boot_upgrade_failure are the
// detective ones, and would have named this incident the moment it happened.
func (c *Collector) collectBootAssessment() groupResult {
	raw, err := os.ReadFile(c.cfg.BootAssessmentFile)
	if err != nil {
		// Absence is not "unarmed": on a node where COS_STATE is not mounted
		// we simply cannot tell, so fail the group rather than report a
		// reassuring zero.
		c.debugf("read boot assessment env failed: %v", err)
		return groupResult{success: false}
	}
	env := parseGRUBEnv(string(raw))
	return groupResult{
		success: true,
		samples: []sample{
			{desc: c.bootAssessArmed, value: boolValue(strings.EqualFold(env["boot_assessment_tentative"], "yes"))},
			{desc: c.bootAssessOn, value: boolValue(strings.EqualFold(env["enable_boot_assessment"], "yes"))},
			{desc: c.bootPassive, value: boolValue(fileExists(filepath.Join(c.cfg.RunCosDir, "passive_mode")))},
			{desc: c.bootUpgradeFail, value: boolValue(fileExists(filepath.Join(c.cfg.RunCosDir, "upgrade_failure")))},
		},
	}
}

// parseGRUBEnv reads GRUB's env block format: key=value lines followed by a
// long run of '#' padding that keeps the file a fixed size.
func parseGRUBEnv(content string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\x00"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		env[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return env
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func readBuildMetadata(path string) (map[string]string, error) {
	return buildmetadata.Read(path)
}

// Keep this split aligned with flavorFamily in tools/internal/image/plan/plan.go;
// the separate Go modules cannot share the helper without a new shared module.
func splitFlavor(flavor string) (family string, version string) {
	family, version, found := strings.Cut(flavor, "-")
	if !found || family == "" {
		return flavor, ""
	}
	return family, version
}

// renderExposition emits gauges grouped by family, HELP/TYPE once each,
// families sorted by name for deterministic output.
func renderExposition(samples []sample) string {
	byFamily := map[*desc][]sample{}
	var order []*desc
	for _, s := range samples {
		if _, seen := byFamily[s.desc]; !seen {
			order = append(order, s.desc)
		}
		byFamily[s.desc] = append(byFamily[s.desc], s)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].name < order[j].name })
	var b strings.Builder
	for _, d := range order {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n", d.name, d.help, d.name)
		for _, s := range byFamily[d] {
			if len(s.labels) != len(d.labels) {
				continue
			}
			b.WriteString(d.name)
			if len(d.labels) > 0 {
				b.WriteByte('{')
				for i, label := range d.labels {
					if i > 0 {
						b.WriteByte(',')
					}
					fmt.Fprintf(&b, "%s=%q", label, s.labels[i])
				}
				b.WriteByte('}')
			}
			fmt.Fprintf(&b, " %s\n", formatValue(s.value))
		}
	}
	return b.String()
}

func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// writeTextfile replaces path atomically: the node_exporter textfile
// collector reads on its own schedule and must never see a partial file.
func writeTextfile(path string, content string) error {
	return textfile.Write(path, content)
}

func (c *Collector) collectZFSPools() groupResult {
	out, err := c.run.Output("zpool", "list", "-Hp", "-o", "name,size,alloc,frag,cap,health")
	if err != nil {
		c.debugf("zpool list failed: %v", err)
		return groupResult{success: false}
	}
	result := groupResult{success: true}
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) != 6 {
			result.success = false
			continue
		}
		size, okSize := parseFloat(fields[1])
		alloc, okAlloc := parseFloat(fields[2])
		frag, okFrag := parseRatio(fields[3])
		capacity, okCap := parseRatio(fields[4])
		if !okSize || !okAlloc || !okFrag || !okCap {
			result.success = false
			continue
		}
		healthValue := 0.0
		if fields[5] == "ONLINE" {
			healthValue = 1
		}
		pool := fields[0]
		result.samples = append(result.samples,
			sample{desc: c.zfsPoolHealth, value: healthValue, labels: []string{pool}},
			sample{desc: c.zfsPoolSize, value: size, labels: []string{pool}},
			sample{desc: c.zfsPoolAlloc, value: alloc, labels: []string{pool}},
			sample{desc: c.zfsPoolFrag, value: frag, labels: []string{pool}},
			sample{desc: c.zfsPoolCap, value: capacity, labels: []string{pool}},
		)
	}
	return result
}

func (c *Collector) collectZFSKeyStatus() groupResult {
	pools, err := c.run.Output("zpool", "list", "-Hp", "-o", "name")
	if err != nil {
		c.debugf("zpool list for keystatus failed: %v", err)
		return groupResult{success: false}
	}
	result := groupResult{success: true}
	for _, pool := range strings.Fields(pools) {
		out, err := c.run.Output("zfs", "get", "-Hp", "-o", "name,value", "keystatus", "-r", "-t", "filesystem", pool)
		if err != nil {
			result.success = false
			c.debugf("zfs keystatus for %s failed: %v", pool, err)
			continue
		}
		for _, line := range lines(out) {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				result.success = false
				continue
			}
			if fields[1] == "-" {
				continue
			}
			value := 0.0
			if fields[1] == "available" {
				value = 1
			}
			result.samples = append(result.samples, sample{desc: c.zfsKeyStatus, value: value, labels: []string{fields[0]}})
		}
	}
	return result
}

func (c *Collector) collectZFSVolumes() groupResult {
	out, err := c.run.Output("zfs", "list", "-Hp", "-t", "volume", "-o", "name,volsize,used")
	if err != nil {
		c.debugf("zfs volume list failed: %v", err)
		return groupResult{success: false}
	}
	result := groupResult{success: true}
	count := 0
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			result.success = false
			continue
		}
		size, okSize := parseFloat(fields[1])
		used, okUsed := parseFloat(fields[2])
		if !okSize || !okUsed {
			result.success = false
			continue
		}
		count++
		result.samples = append(result.samples,
			sample{desc: c.zfsVolumeSize, value: size, labels: []string{fields[0]}},
			sample{desc: c.zfsVolumeUsed, value: used, labels: []string{fields[0]}},
		)
	}
	result.samples = append(result.samples, sample{desc: c.zfsVolumes, value: float64(count)})
	return result
}

func (c *Collector) collectLIO() groupResult {
	targets, luns, sessions, err := countLIO(c.cfg.ConfigFSRoot)
	if err != nil {
		c.debugf("read LIO configfs failed: %v", err)
		return groupResult{success: false}
	}
	inSync := 0.0
	if saveConfigInSync(c.cfg.SaveConfig, targets) {
		inSync = 1
	}
	return groupResult{
		success: true,
		samples: []sample{
			{desc: c.lioTargets, value: float64(targets)},
			{desc: c.lioLUNs, value: float64(luns)},
			{desc: c.lioSessions, value: float64(sessions)},
			{desc: c.lioSaveInSync, value: inSync},
		},
	}
}

func countLIO(root string) (targets, luns, sessions int, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "iqn.") {
			continue
		}
		targets++
		tpgtEntries, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return 0, 0, 0, err
		}
		for _, tpgt := range tpgtEntries {
			if !tpgt.IsDir() || !strings.HasPrefix(tpgt.Name(), "tpgt_") {
				continue
			}
			luns += countDirs(filepath.Join(root, entry.Name(), tpgt.Name(), "lun"), "lun_")
			// ponytail: ACL dirs are enough for the D28 boot/e2e signal. They are cached
			// (democratic-csi sets cache_dynamic_acls), so they outlive the sessions that created them.
			sessions += countDirs(filepath.Join(root, entry.Name(), tpgt.Name(), "acls"), "")
		}
	}
	return targets, luns, sessions, nil
}

func countDirs(path string, prefix string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			count++
		}
	}
	return count
}

// saveConfigInSync compares only the NUMBER of live targets against the number
// in saveconfig.json — it does not compare target identities, TPGs, LUNs or
// ACLs. An absent saveconfig.json is reported as out of sync rather than
// trivially in sync: nothing would be restored at the next boot, and with an
// unreadable configfs (live count 0) "both absent" would otherwise read as a
// clean bill of health for a box whose LIO state is simply unknown.
func saveConfigInSync(path string, liveTargets int) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var parsed struct {
		Targets []json.RawMessage `json:"targets"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false
	}
	return len(parsed.Targets) == liveTargets
}

func (c *Collector) collectSMART() groupResult {
	out, err := c.run.Output("smartctl", "--scan", "-j")
	if err != nil {
		c.debugf("smartctl scan skipped: %v", err)
		return groupResult{success: false}
	}
	var scan struct {
		Devices []struct {
			Name string `json:"name"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(out), &scan); err != nil {
		c.debugf("smartctl scan JSON unparseable: %v", err)
		return groupResult{success: false}
	}
	result := groupResult{success: true}
	for _, device := range scan.Devices {
		if device.Name == "" {
			continue
		}
		devOut, err := c.run.Output("smartctl", "-aj", device.Name)
		if err != nil {
			c.debugf("smartctl skipped %s: %v", device.Name, err)
			continue
		}
		metrics, ok := c.parseSMARTDevice(devOut)
		if !ok {
			c.debugf("smartctl skipped %s: no supported SMART fields", device.Name)
			continue
		}
		for _, metric := range metrics {
			metric.labels = []string{device.Name}
			result.samples = append(result.samples, metric)
		}
	}
	return result
}

func (c *Collector) collectStorageHealth() groupResult {
	info, err := os.Stat(c.cfg.StatusFile)
	if err != nil {
		if !os.IsNotExist(err) {
			c.debugf("stat storage health status failed: %v", err)
			return groupResult{success: false}
		}
		return groupResult{success: true, samples: []sample{
			{desc: c.storageHealthy, value: 0},
			{desc: c.storageLastRun, value: 0},
		}}
	}
	data, err := os.ReadFile(c.cfg.StatusFile)
	if err != nil {
		c.debugf("read storage health status failed: %v", err)
		return groupResult{success: false}
	}
	healthy := 0.0
	if strings.TrimSuffix(firstWord(string(data)), ":") == "healthy" {
		healthy = 1
	}
	return groupResult{success: true, samples: []sample{
		{desc: c.storageHealthy, value: healthy},
		{desc: c.storageLastRun, value: float64(info.ModTime().Unix())},
	}}
}

func (c *Collector) parseSMARTDevice(out string) ([]sample, bool) {
	var doc struct {
		Temperature *struct {
			Current *float64 `json:"current"`
		} `json:"temperature"`
		NVMe *struct {
			PercentageUsed *float64 `json:"percentage_used"`
			MediaErrors    *float64 `json:"media_errors"`
		} `json:"nvme_smart_health_information_log"`
		PowerOnTime *struct {
			Hours *float64 `json:"hours"`
		} `json:"power_on_time"`
		ATAErrorLog *struct {
			Summary *struct {
				Count *float64 `json:"count"`
			} `json:"summary"`
		} `json:"ata_smart_error_log"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, false
	}
	var samples []sample
	if doc.Temperature != nil && doc.Temperature.Current != nil {
		samples = append(samples, sample{desc: c.smartTemp, value: *doc.Temperature.Current})
	}
	if doc.NVMe != nil {
		if doc.NVMe.PercentageUsed != nil {
			samples = append(samples, sample{desc: c.smartPctUsed, value: *doc.NVMe.PercentageUsed})
		}
		if doc.NVMe.MediaErrors != nil {
			samples = append(samples, sample{desc: c.smartMediaErrors, value: *doc.NVMe.MediaErrors})
		}
	}
	if doc.ATAErrorLog != nil && doc.ATAErrorLog.Summary != nil && doc.ATAErrorLog.Summary.Count != nil {
		samples = append(samples, sample{desc: c.smartMediaErrors, value: *doc.ATAErrorLog.Summary.Count})
	}
	if doc.PowerOnTime != nil && doc.PowerOnTime.Hours != nil {
		samples = append(samples, sample{desc: c.smartPowerHours, value: *doc.PowerOnTime.Hours})
	}
	return samples, len(samples) > 0
}

func normalize(cfg Config) Config {
	if cfg.TextfileDir == "" {
		cfg.TextfileDir = DefaultTextfileDir
	}
	if cfg.ConfigFSRoot == "" {
		cfg.ConfigFSRoot = defaultConfigFSRoot
	}
	if cfg.SaveConfig == "" {
		cfg.SaveConfig = defaultSaveConfig
	}
	if cfg.StatusFile == "" {
		cfg.StatusFile = health.DefaultStatusFile
	}
	if cfg.MetadataFile == "" {
		cfg.MetadataFile = buildmetadata.DefaultPath
	}
	if cfg.BootAssessmentFile == "" {
		cfg.BootAssessmentFile = defaultBootAssessment
	}
	if cfg.RunCosDir == "" {
		cfg.RunCosDir = defaultRunCosDir
	}
	if cfg.Debug == nil {
		cfg.Debug = io.Discard
	}
	return cfg
}

func (c *Collector) debugf(format string, args ...any) {
	if c.cfg.Debug == nil {
		return
	}
	_, _ = fmt.Fprintf(c.cfg.Debug, "k2-node-agent metrics: "+format+"\n", args...)
}

func parseFloat(s string) (float64, bool) {
	value, err := strconv.ParseFloat(s, 64)
	return value, err == nil
}

// parseRatio converts zpool's frag/cap fields — integer percent under -Hp
// (some releases keep a % suffix) — to a 0-1 ratio. Always divide: a
// "guess the unit by magnitude" heuristic misreads 1% as a 1.0 ratio,
// exactly at the capacity-alert boundary.
func parseRatio(s string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil {
		return 0, false
	}
	return value / 100, true
}

func lines(out string) []string {
	var result []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// cadenceSnapshotPattern matches the k2-node-agent snapshot naming scheme
// (<prefix>-<UTC stamp>); manual and migration snapshots never match, so
// they don't pollute the cadence gauges.
var cadenceSnapshotPattern = regexp.MustCompile(`^(.+)-(\d{8}T\d{6}Z)$`)

// collectZFSSnapshots aggregates cadence snapshots by prefix. A recursive
// snapshot stamps the same @name onto every child dataset, so distinct
// snapshot suffixes — not raw rows — are what count as retention points.
func (c *Collector) collectZFSSnapshots() groupResult {
	out, err := c.run.Output("zfs", "list", "-Hp", "-t", "snapshot", "-o", "name,creation")
	if err != nil {
		c.debugf("zfs snapshot list failed: %v", err)
		return groupResult{success: false}
	}
	type agg struct {
		suffixes map[string]bool
		last     float64
	}
	byPrefix := map[string]*agg{}
	result := groupResult{success: true}
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			result.success = false
			continue
		}
		_, snap, found := strings.Cut(fields[0], "@")
		if !found {
			result.success = false
			continue
		}
		match := cadenceSnapshotPattern.FindStringSubmatch(snap)
		if match == nil {
			continue
		}
		creation, ok := parseFloat(fields[1])
		if !ok {
			result.success = false
			continue
		}
		prefix := match[1]
		a := byPrefix[prefix]
		if a == nil {
			a = &agg{suffixes: map[string]bool{}}
			byPrefix[prefix] = a
		}
		a.suffixes[match[2]] = true
		a.last = max(a.last, creation)
	}
	prefixes := make([]string, 0, len(byPrefix))
	for prefix := range byPrefix {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	for _, prefix := range prefixes {
		a := byPrefix[prefix]
		result.samples = append(result.samples,
			sample{desc: c.snapshotLast, value: a.last, labels: []string{prefix}},
			sample{desc: c.snapshotCount, value: float64(len(a.suffixes)), labels: []string{prefix}},
		)
	}
	return result
}
