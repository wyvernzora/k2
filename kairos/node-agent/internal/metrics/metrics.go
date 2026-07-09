package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/health"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/runner"
)

const (
	DefaultListen       = "0.0.0.0:9110"
	defaultConfigFSRoot = "/sys/kernel/config/target/iscsi"
	defaultSaveConfig   = "/etc/rtslib-fb-target/saveconfig.json"

	cheapTTL = 10 * time.Second
	smartTTL = 5 * time.Minute
)

type Config struct {
	Listen       string
	ConfigFSRoot string
	SaveConfig   string
	StatusFile   string
	Debug        io.Writer
}

type Collector struct {
	cfg   Config
	run   runner.Runner
	now   func() time.Time
	mu    sync.Mutex
	cache map[string]cachedGroup

	collectorSuccess *prometheus.Desc
	zfsPoolHealth    *prometheus.Desc
	zfsPoolSize      *prometheus.Desc
	zfsPoolAlloc     *prometheus.Desc
	zfsPoolFrag      *prometheus.Desc
	zfsPoolCap       *prometheus.Desc
	zfsKeyStatus     *prometheus.Desc
	zfsVolumeSize    *prometheus.Desc
	zfsVolumeUsed    *prometheus.Desc
	zfsVolumes       *prometheus.Desc
	lioTargets       *prometheus.Desc
	lioLUNs          *prometheus.Desc
	lioSessions      *prometheus.Desc
	lioSaveInSync    *prometheus.Desc
	smartTemp        *prometheus.Desc
	smartPctUsed     *prometheus.Desc
	smartMediaErrors *prometheus.Desc
	smartPowerHours  *prometheus.Desc
	storageHealthy   *prometheus.Desc
	storageLastRun   *prometheus.Desc
}

type cachedGroup struct {
	expires time.Time
	result  groupResult
}

type groupResult struct {
	samples []sample
	success bool
}

type sample struct {
	desc   *prometheus.Desc
	value  float64
	labels []string
}

func Run(cfg Config) error {
	cfg = normalize(cfg)
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler(cfg, runner.OSRunner{}))
	return http.ListenAndServe(cfg.Listen, mux)
}

func Handler(cfg Config, run runner.Runner) http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewCollector(cfg, run))
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

func NewCollector(cfg Config, run runner.Runner) *Collector {
	cfg = normalize(cfg)
	return &Collector{
		cfg:   cfg,
		run:   run,
		now:   time.Now,
		cache: map[string]cachedGroup{},

		collectorSuccess: prometheus.NewDesc("k2_collector_success", "Whether the K2 storage metrics collector group succeeded.", []string{"collector"}, nil),
		zfsPoolHealth:    prometheus.NewDesc("k2_zfs_pool_health", "ZFS pool health, 1 when ONLINE.", []string{"pool"}, nil),
		zfsPoolSize:      prometheus.NewDesc("k2_zfs_pool_size_bytes", "ZFS pool size in bytes.", []string{"pool"}, nil),
		zfsPoolAlloc:     prometheus.NewDesc("k2_zfs_pool_alloc_bytes", "ZFS pool allocated bytes.", []string{"pool"}, nil),
		zfsPoolFrag:      prometheus.NewDesc("k2_zfs_pool_fragmentation_ratio", "ZFS pool fragmentation ratio.", []string{"pool"}, nil),
		zfsPoolCap:       prometheus.NewDesc("k2_zfs_pool_capacity_ratio", "ZFS pool capacity ratio.", []string{"pool"}, nil),
		zfsKeyStatus:     prometheus.NewDesc("k2_zfs_keystatus_available", "ZFS encrypted dataset key availability.", []string{"dataset"}, nil),
		zfsVolumeSize:    prometheus.NewDesc("k2_zfs_volume_size_bytes", "ZFS volume size in bytes.", []string{"volume"}, nil),
		zfsVolumeUsed:    prometheus.NewDesc("k2_zfs_volume_used_bytes", "ZFS volume used bytes.", []string{"volume"}, nil),
		zfsVolumes:       prometheus.NewDesc("k2_zfs_volumes", "Total ZFS volume count.", nil, nil),
		lioTargets:       prometheus.NewDesc("k2_lio_targets", "LIO iSCSI target count.", nil, nil),
		lioLUNs:          prometheus.NewDesc("k2_lio_luns", "LIO LUN count.", nil, nil),
		lioSessions:      prometheus.NewDesc("k2_lio_sessions", "LIO active session count.", nil, nil),
		lioSaveInSync:    prometheus.NewDesc("k2_lio_saveconfig_in_sync", "Whether live LIO target count matches saveconfig.", nil, nil),
		smartTemp:        prometheus.NewDesc("k2_smart_temperature_celsius", "SMART temperature in Celsius.", []string{"device"}, nil),
		smartPctUsed:     prometheus.NewDesc("k2_smart_percentage_used", "NVMe SMART percentage used.", []string{"device"}, nil),
		smartMediaErrors: prometheus.NewDesc("k2_smart_media_errors", "SMART media error count.", []string{"device"}, nil),
		smartPowerHours:  prometheus.NewDesc("k2_smart_power_on_hours", "SMART power-on hours.", []string{"device"}, nil),
		storageHealthy:   prometheus.NewDesc("k2_storage_healthy", "K2 storage health status.", nil, nil),
		storageLastRun:   prometheus.NewDesc("k2_storage_health_last_run_timestamp_seconds", "Unix timestamp of the last storage health status write.", nil, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{
		c.collectorSuccess, c.zfsPoolHealth, c.zfsPoolSize, c.zfsPoolAlloc, c.zfsPoolFrag, c.zfsPoolCap,
		c.zfsKeyStatus, c.zfsVolumeSize, c.zfsVolumeUsed, c.zfsVolumes, c.lioTargets, c.lioLUNs, c.lioSessions,
		c.lioSaveInSync, c.smartTemp, c.smartPctUsed, c.smartMediaErrors, c.smartPowerHours, c.storageHealthy, c.storageLastRun,
	} {
		ch <- desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	for _, group := range []struct {
		name string
		ttl  time.Duration
		fn   func() groupResult
	}{
		{"zfs_pools", cheapTTL, c.collectZFSPools},
		{"zfs_keystatus", cheapTTL, c.collectZFSKeyStatus},
		{"zfs_volumes", cheapTTL, c.collectZFSVolumes},
		{"lio", cheapTTL, c.collectLIO},
		{"smart", smartTTL, c.collectSMART},
		{"storage_health", cheapTTL, c.collectStorageHealth},
	} {
		result := c.cached(group.name, group.ttl, group.fn)
		for _, sample := range result.samples {
			ch <- prometheus.MustNewConstMetric(sample.desc, prometheus.GaugeValue, sample.value, sample.labels...)
		}
		success := 0.0
		if result.success {
			success = 1
		}
		ch <- prometheus.MustNewConstMetric(c.collectorSuccess, prometheus.GaugeValue, success, group.name)
	}
}

func (c *Collector) cached(name string, ttl time.Duration, fn func() groupResult) groupResult {
	now := c.now()
	c.mu.Lock()
	entry, ok := c.cache[name]
	if ok && now.Before(entry.expires) {
		c.mu.Unlock()
		return entry.result
	}
	c.mu.Unlock()

	result := fn()

	c.mu.Lock()
	c.cache[name] = cachedGroup{expires: now.Add(ttl), result: result}
	c.mu.Unlock()
	return result
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

func countLIO(root string) (int, int, int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, err
	}
	targets, luns, sessions := 0, 0, 0
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
			// ponytail: configfs session state is deeper; ACL dirs are enough for the D28 boot/e2e signal.
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

func saveConfigInSync(path string, liveTargets int) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return os.IsNotExist(err) && liveTargets == 0
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
	if cfg.Listen == "" {
		cfg.Listen = DefaultListen
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
