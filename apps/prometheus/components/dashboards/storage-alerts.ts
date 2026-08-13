import {
  PrometheusRule,
  type PrometheusRuleProps,
  PrometheusRuleSpecGroupsRulesExpr as RuleExpr,
} from "../../crds/monitoring.coreos.com.js";

/**
 * Alerting for the storage appliance, in two layers.
 *
 * The leaf alerts below each name one condition and carry `k2_component:
 * storage-appliance`. The rollup alert fires when ANY leaf is firing, so
 * notification routing has a single stable target to attach to rather than a
 * list that must be edited every time a leaf is added. Routing itself is not
 * configured yet — the rollup is the seam it will plug into.
 *
 * Two deliberate choices, both from what the review rounds surfaced:
 *
 * 1. Every leaf that reads a gauge is paired with an `absent()` leaf. A
 *    stopped writer, an unreachable appliance, or a collector that silently
 *    stopped emitting produces NO series — and a threshold over a missing
 *    series never fires. Absence is a distinct alarm, not silence.
 * 2. The appliance is scraped as job "storage-appliance", so NONE of
 *    kube-prometheus-stack's bundled Node* alerts (which pin
 *    job="node-exporter") apply to it. Anything the appliance needs alerted
 *    on has to be here.
 */
export class StorageAlerts extends PrometheusRule {
  public constructor(scope: ConstructScope, id: string) {
    super(scope, id, storageAlertRules());
  }
}

// Local alias so this file does not depend on the constructs typing surface
// beyond what PrometheusRule already requires.
type ConstructScope = ConstructorParameters<typeof PrometheusRule>[0];

const APPLIANCE = 'job="storage-appliance",instance="k2-st-0e12"';
const COMPONENT = { k2_component: "storage-appliance" };

function storageAlertRules(): PrometheusRuleProps {
  return {
    metadata: { name: "storage-appliance-alerts" },
    spec: { groups: [leafGroup(), rollupGroup()] },
  };
}

function leafGroup() {
  return {
    name: "k2.storage.appliance",
    rules: [
      // Scrape liveness first: if this fires the others may be absent rather
      // than false, which is why absence alerts below carry a longer `for`.
      alert(
        "StorageApplianceDown",
        `up{${APPLIANCE}} == 0`,
        "5m",
        "critical",
        "Storage appliance is not scrapeable",
        "Prometheus cannot reach k2-st-0e12:9100. iSCSI may still be serving — this is a monitoring-path alarm, not proof of data-path failure.",
      ),

      alert(
        "StorageApplianceMetricsAbsent",
        `absent(up{${APPLIANCE}})`,
        "15m",
        "critical",
        "Storage appliance target has disappeared",
        "The scrape target itself is gone, not merely down — the ScrapeConfig may have been removed or relabelled.",
      ),

      alert(
        "StoragePoolNotOnline",
        `k2_zfs_pool_health{${APPLIANCE}} != 1`,
        "5m",
        "critical",
        "ZFS pool {{ $labels.pool }} is not ONLINE",
        "A vdev is degraded or faulted. Every PVC on this appliance is affected.",
      ),

      alert(
        "StoragePoolKeyUnavailable",
        `k2_zfs_keystatus_available{${APPLIANCE}} != 1`,
        "5m",
        "critical",
        "ZFS encryption key not loaded for {{ $labels.dataset }}",
        "The pool is imported but locked, so no zvol can be served. Usually means the boot key-load stage failed.",
      ),

      alert(
        "StoragePoolNearFull",
        `k2_zfs_pool_capacity_ratio{${APPLIANCE}} > 0.8`,
        "30m",
        "warning",
        "ZFS pool {{ $labels.pool }} is over 80% full",
        "ZFS performance degrades well before 100%; expand or reclaim before it becomes urgent.",
      ),

      // The collector's own health. Without this a broken collector looks
      // identical to a healthy-but-quiet subsystem.
      alert(
        "StorageCollectorFailing",
        `k2_collector_success{${APPLIANCE}} == 0`,
        "15m",
        "warning",
        "Storage collector {{ $labels.collector }} is failing",
        "The metrics this collector owns are stale or missing; treat panels fed by it as unknown, not healthy.",
      ),

      alert(
        "StorageHealthCheckStale",
        `time() - k2_storage_health_last_run_timestamp_seconds{${APPLIANCE}} > 7200`,
        "0m",
        "warning",
        "Storage health check has not run for over 2 hours",
        "k2-storage-health.timer appears stopped; appliance health is unobserved.",
      ),

      // Backup cadence. Absence is the real risk here: a pool that has never
      // been snapshotted emits no series at all, so a staleness threshold
      // alone would stay silent forever.
      // Thresholds are per-cadence. A single 3h bound fired against prefix
      // "k2-daily" for 21 hours out of every 24 — unsatisfiable by
      // construction, and it pinned the StorageApplianceDegraded rollup so a
      // real fault could not be seen. Each bound is ~3x its own interval, so a
      // timer has to miss several runs before it trips.
      alert(
        "StorageSnapshotStale",
        `time() - k2_zfs_last_snapshot_timestamp_seconds{${APPLIANCE},prefix="k2-hourly"} > 10800`,
        "0m",
        "warning",
        "No hourly snapshot for over 3 hours",
        "The appliance hourly snapshot timer has stopped or is failing; recent writes are unprotected.",
      ),

      alert(
        "StorageDailySnapshotStale",
        `time() - k2_zfs_last_snapshot_timestamp_seconds{${APPLIANCE},prefix="k2-daily"} > 259200`,
        "0m",
        "warning",
        "No daily snapshot for over 3 days",
        "The appliance daily snapshot timer has stopped or is failing; the long-horizon restore points are not being created.",
      ),

      alert(
        "StorageSnapshotSeriesAbsent",
        `absent(k2_zfs_last_snapshot_timestamp_seconds{${APPLIANCE}})`,
        "30m",
        "warning",
        "No snapshot cadence series at all",
        "Not a stale snapshot — NO cadence snapshot exists. The timers may never have run on this appliance.",
      ),

      // Device health.
      alert(
        "StorageDeviceMediaErrors",
        `k2_smart_media_errors{${APPLIANCE}} > 0`,
        "5m",
        "warning",
        "SMART media errors on {{ $labels.device }}",
        "Media errors are cumulative and never decrease; investigate before the mirror loses redundancy.",
      ),

      alert(
        "StorageDeviceWearHigh",
        `k2_smart_percentage_used{${APPLIANCE}} > 80`,
        "1h",
        "warning",
        "NVMe wear on {{ $labels.device }} above 80%",
        "Plan replacement; replace one mirror member at a time and let it resilver fully.",
      ),

      // Image currency. check_success is alerted separately from
      // upgrade_available so a failed check can never read as "up to date".
      alert(
        "StorageImageCheckFailing",
        `k2_node_image_check_success{${APPLIANCE}} == 0`,
        "3h",
        "warning",
        "Image upgrade check is failing",
        "The node cannot reach the registry, so k2_node_image_upgrade_available is unknown — not zero.",
      ),

      alert(
        "StorageImageCheckStale",
        `time() - k2_node_image_check_timestamp_seconds{${APPLIANCE}} > 10800`,
        "0m",
        "warning",
        "Image upgrade check has not run for over 3 hours",
        "k2-image-check.timer appears stopped; image currency is unobserved.",
      ),
    ],
  };
}

/**
 * The aggregation the operator asked for: one alert that fires when any leaf
 * above is firing, carrying the worst severity present. Notification routing
 * attaches here, so adding a leaf never means touching the routing config.
 */
function rollupGroup() {
  return {
    name: "k2.storage.appliance.rollup",
    rules: [
      {
        alert: "StorageApplianceDegraded",
        // k2_rollup!="true" excludes this alert from its own count. Without it
        // the rollup carries k2_component=storage-appliance into ALERTS, keeps
        // matching itself, and stays firing forever after the leaf that first
        // tripped it has cleared.
        expr: RuleExpr.fromString(
          `count(ALERTS{k2_component="storage-appliance",k2_rollup!="true",alertstate="firing"}) > 0`,
        ),
        for: "0m",
        labels: { ...COMPONENT, severity: "warning", k2_rollup: "true" },
        annotations: {
          summary: "Storage appliance has {{ $value }} active alert(s)",
          description:
            "Aggregate for k2-st-0e12. Individual causes are the firing ALERTS with k2_component=storage-appliance; this exists so notification routing has one target that does not change as leaf alerts are added.",
        },
      },
    ],
  };
}

function alert(
  name: string,
  expr: string,
  forDuration: string,
  severity: string,
  summary: string,
  description: string,
) {
  return {
    alert: name,
    expr: RuleExpr.fromString(expr),
    for: forDuration,
    labels: { ...COMPONENT, severity },
    annotations: { summary, description },
  };
}
