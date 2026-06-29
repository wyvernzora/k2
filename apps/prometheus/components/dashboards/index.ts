import { ConfigMap } from "cdk8s-plus-32";
import { Construct } from "constructs";

export const DASHBOARD_FOLDER_ANNOTATION = "k8s-sidecar-target-directory";
export const DASHBOARD_ROOT = "/tmp/dashboards";

const DASHBOARD_LABELS = {
  grafana_dashboard: "1",
};

const DATASOURCE = {
  type: "prometheus",
  uid: "prometheus",
};

export class GrafanaDashboards extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    for (const dashboard of dashboards()) {
      new ConfigMap(this, dashboard.uid, {
        metadata: {
          name: `k2-grafana-dashboard-${dashboard.uid}`,
          labels: DASHBOARD_LABELS,
          annotations: {
            [DASHBOARD_FOLDER_ANNOTATION]: `${DASHBOARD_ROOT}/${dashboard.folder}`,
          },
        },
        data: {
          [`${dashboard.uid}.json`]: JSON.stringify(dashboard.definition, null, 2),
        },
      });
    }
  }
}

interface DashboardSpec {
  readonly uid: string;
  readonly folder: string;
  readonly definition: Record<string, unknown>;
}

interface PanelTarget {
  readonly expr: string;
  readonly legendFormat?: string;
}

function dashboards(): DashboardSpec[] {
  return [
    dashboard("k2-scrape-targets", "Applications", "K2 / Scrape Targets", [
      stat(1, "Scrape targets up", "sum(up)", 0, 0, "short"),
      stat(2, "Scrape targets down", "sum(up == 0)", 6, 0, "short"),
      stat(3, "Scraped samples / sec", "sum(rate(scrape_samples_scraped[$__rate_interval]))", 12, 0, "ops"),
      stat(4, "Avg scrape duration", "avg(scrape_duration_seconds)", 18, 0, "s"),
      timeSeries(
        5,
        "Target health by job",
        [{ expr: "sum by (namespace, job) (up)", legendFormat: "{{namespace}} / {{job}}" }],
        0,
        4,
      ),
      timeSeries(
        6,
        "Scraped samples by job",
        [
          {
            expr: "sum by (namespace, job) (rate(scrape_samples_scraped[$__rate_interval]))",
            legendFormat: "{{namespace}} / {{job}}",
          },
        ],
        0,
        12,
        "ops",
      ),
    ]),
    dashboard("k2-argocd", "Applications", "K2 / Argo CD", [
      stat(1, "Applications", "count(argocd_app_info)", 0, 0, "short"),
      stat(2, "Out of sync", 'sum(argocd_app_sync_status{sync_status!="Synced"})', 6, 0, "short"),
      stat(3, "Unhealthy", 'sum(argocd_app_health_status{health_status!="Healthy"})', 12, 0, "short"),
      stat(4, "Reconciles / sec", "sum(rate(argocd_app_reconcile_count[$__rate_interval]))", 18, 0, "ops"),
      timeSeries(
        5,
        "Reconcile rate",
        [
          {
            expr: "sum by (namespace, name) (rate(argocd_app_reconcile_count[$__rate_interval]))",
            legendFormat: "{{namespace}} / {{name}}",
          },
        ],
        0,
        4,
        "ops",
      ),
      timeSeries(
        6,
        "Sync operations",
        [{ expr: "sum by (phase) (rate(argocd_app_sync_total[$__rate_interval]))", legendFormat: "{{phase}}" }],
        0,
        12,
        "ops",
      ),
    ]),
    takuhaiDashboard(),
    dashboard("k2-dns", "Networking", "K2 / DNS", [
      stat(1, "DNS queries / sec", "sum(rate(blocky_query_total[$__rate_interval]))", 0, 0, "ops"),
      stat(2, "DNS errors / sec", "sum(rate(blocky_error_total[$__rate_interval]))", 6, 0, "ops"),
      stat(
        3,
        "Cache hit ratio",
        "sum(rate(blocky_cache_hits_total[$__rate_interval])) / clamp_min(sum(rate(blocky_cache_hits_total[$__rate_interval])) + sum(rate(blocky_cache_misses_total[$__rate_interval])), 1)",
        12,
        0,
        "percentunit",
      ),
      stat(
        4,
        "Blocked responses / sec",
        'sum(rate(blocky_response_total{response_type="BLOCKED"}[$__rate_interval]))',
        18,
        0,
        "ops",
      ),
      timeSeries(
        5,
        "DNS query volume",
        [{ expr: "sum by (type) (rate(blocky_query_total[$__rate_interval]))", legendFormat: "{{type}}" }],
        0,
        4,
        "ops",
      ),
      timeSeries(
        6,
        "DNS response volume",
        [
          {
            expr: "sum by (response_type) (rate(blocky_response_total[$__rate_interval]))",
            legendFormat: "{{response_type}}",
          },
        ],
        0,
        12,
        "ops",
      ),
    ]),
    dashboard("k2-security-controllers", "Security", "K2 / Certificates and Secrets", [
      stat(1, "Ready certificates", 'sum(certmanager_certificate_ready_status{condition="True"})', 0, 0, "short"),
      stat(
        2,
        "Certificate sync errors / sec",
        "sum(rate(certmanager_controller_sync_error_count[$__rate_interval]))",
        6,
        0,
        "ops",
      ),
      stat(
        3,
        "ExternalSecret syncs / sec",
        "sum(rate(externalsecret_sync_calls_total[$__rate_interval]))",
        12,
        0,
        "ops",
      ),
      stat(
        4,
        "ExternalSecret errors / sec",
        "sum(rate(externalsecret_sync_calls_error[$__rate_interval]))",
        18,
        0,
        "ops",
      ),
      timeSeries(
        5,
        "Certificate expiration",
        [
          {
            expr: "(certmanager_certificate_expiration_timestamp_seconds - time()) / 86400",
            legendFormat: "{{namespace}} / {{name}}",
          },
        ],
        0,
        4,
        "d",
      ),
      timeSeries(
        6,
        "ExternalSecret status",
        [
          {
            expr: "sum by (namespace, name, condition, status) (externalsecret_status_condition)",
            legendFormat: "{{namespace}} / {{name}} {{condition}}={{status}}",
          },
        ],
        0,
        12,
      ),
    ]),
    dashboard("k2-networking", "Networking", "K2 / Cilium and Pomerium", [
      stat(
        1,
        "Cilium operator warnings / sec",
        "sum(rate(cilium_operator_errors_warnings_total[$__rate_interval]))",
        0,
        0,
        "ops",
      ),
      stat(2, "LB IPs used", "sum(cilium_operator_lbipam_ips_used)", 6, 0, "short"),
      stat(3, "LB IPs available", "sum(cilium_operator_lbipam_ips_available)", 12, 0, "short"),
      stat(
        4,
        "Pomerium upstream req / sec",
        "sum(rate(envoy_cluster_upstream_rq_total[$__rate_interval]))",
        18,
        0,
        "ops",
      ),
      timeSeries(
        5,
        "Cilium workqueue depth",
        [{ expr: "sum by (name) (cilium_k8s_workqueue_depth)", legendFormat: "{{name}}" }],
        0,
        4,
      ),
      timeSeries(
        6,
        "Pomerium upstream responses",
        [
          {
            expr: "sum by (envoy_response_code_class) (rate(envoy_cluster_upstream_rq_xx[$__rate_interval]))",
            legendFormat: "{{envoy_response_code_class}}",
          },
        ],
        0,
        12,
        "ops",
      ),
    ]),
    dashboard("k2-storage-databases", "Storage", "K2 / Storage and Databases", [
      stat(1, "Longhorn volumes", "count(longhorn_volume_state)", 0, 0, "short"),
      stat(2, "Longhorn nodes ready", 'sum(longhorn_node_status{condition="Ready",status="True"})', 6, 0, "short"),
      stat(3, "Postgres collectors up", "sum(cnpg_collector_up)", 12, 0, "short"),
      stat(4, "Postgres DB size", "sum(cnpg_pg_database_size_bytes)", 18, 0, "bytes"),
      timeSeries(
        5,
        "Longhorn volume throughput",
        [
          { expr: "sum by (volume) (longhorn_volume_read_throughput)", legendFormat: "{{volume}} read" },
          { expr: "sum by (volume) (longhorn_volume_write_throughput)", legendFormat: "{{volume}} write" },
        ],
        0,
        4,
        "Bps",
      ),
      timeSeries(
        6,
        "Postgres transactions",
        [
          {
            expr: "sum by (datname) (rate(cnpg_pg_stat_database_xact_commit[$__rate_interval]))",
            legendFormat: "{{datname}} commits",
          },
          {
            expr: "sum by (datname) (rate(cnpg_pg_stat_database_xact_rollback[$__rate_interval]))",
            legendFormat: "{{datname}} rollbacks",
          },
        ],
        0,
        12,
        "ops",
      ),
    ]),
  ];
}

function takuhaiDashboard(): DashboardSpec {
  return dashboard("takuhai-overview", "Applications", "Takuhai", [
    stat(1, "Claimable Releases", 'max(takuhai_queue_items{state="claimable"})', 0, 0, "short"),
    stat(2, "Exhausted Releases", 'max(takuhai_queue_items{state="exhausted"})', 6, 0, "short"),
    stat(3, "Known Releases", "max(takuhai_catalog_infohashes)", 12, 0, "short"),
    stat(4, "Matched Refs", "max(takuhai_catalog_refs)", 18, 0, "short"),
    timeSeries(
      5,
      "Submission Rate",
      [
        {
          expr: "sum by (status, result) (rate(takuhai_submit_total[$__rate_interval]))",
          legendFormat: "{{status}} {{result}}",
        },
      ],
      0,
      4,
      "ops",
    ),
    timeSeries(6, "Queue State", [{ expr: "max by (state) (takuhai_queue_items)", legendFormat: "{{state}}" }], 0, 12),
    timeSeries(
      7,
      "Submission Confidence Quantiles",
      [
        {
          expr: "histogram_quantile(0.5, sum by (le, status) (rate(takuhai_submit_confidence_bucket[$__rate_interval])))",
          legendFormat: "p50 {{status}}",
        },
        {
          expr: "histogram_quantile(0.9, sum by (le, status) (rate(takuhai_submit_confidence_bucket[$__rate_interval])))",
          legendFormat: "p90 {{status}}",
        },
      ],
      0,
      20,
    ),
    timeSeries(
      8,
      "Submission Confidence Mean / P99",
      [
        {
          expr: "sum by (status) (rate(takuhai_submit_confidence_sum[$__rate_interval])) / sum by (status) (rate(takuhai_submit_confidence_count[$__rate_interval]))",
          legendFormat: "mean {{status}}",
        },
        {
          expr: "histogram_quantile(0.99, sum by (le, status) (rate(takuhai_submit_confidence_bucket[$__rate_interval])))",
          legendFormat: "p99 {{status}}",
        },
      ],
      0,
      28,
    ),
    timeSeries(
      9,
      "Ingest Post Rate",
      [
        {
          expr: "sum by (source, result) (rate(takuhai_ingest_posts_total[$__rate_interval]))",
          legendFormat: "{{source}} {{result}}",
        },
      ],
      0,
      36,
      "ops",
    ),
    timeSeries(
      10,
      "HTTP Latency",
      [
        {
          expr: "histogram_quantile(0.95, sum by (le, path) (rate(takuhai_http_request_duration_seconds_bucket[$__rate_interval])))",
          legendFormat: "p95 {{path}}",
        },
        {
          expr: "histogram_quantile(0.99, sum by (le, path) (rate(takuhai_http_request_duration_seconds_bucket[$__rate_interval])))",
          legendFormat: "p99 {{path}}",
        },
      ],
      0,
      44,
      "s",
    ),
    timeSeries(
      11,
      "HTTP Error Rate",
      [
        {
          expr: 'sum by (path, status) (rate(takuhai_http_requests_total{status=~"4..|5.."}[$__rate_interval]))',
          legendFormat: "{{path}} {{status}}",
        },
      ],
      0,
      52,
      "ops",
    ),
    timeSeries(
      12,
      "MCP Activity",
      [
        {
          expr: "sum by (tool, result) (rate(takuhai_mcp_tool_calls_total[$__rate_interval]))",
          legendFormat: "{{tool}} {{result}}",
        },
        {
          expr: "sum by (result) (rate(takuhai_mcp_resolve_magnets_infohashes_total[$__rate_interval]))",
          legendFormat: "resolve_magnets {{result}}",
        },
      ],
      0,
      60,
      "ops",
    ),
    timeSeries(
      13,
      "DMHY Crawler Activity",
      [
        {
          expr: "sum by (result) (rate(takuhai_dmhy_crawl_requests_total[$__rate_interval]))",
          legendFormat: "crawl {{result}}",
        },
        {
          expr: "sum(rate(takuhai_dmhy_parse_posts_total[$__rate_interval]))",
          legendFormat: "parsed posts",
        },
      ],
      0,
      68,
      "ops",
    ),
    timeSeries(
      14,
      "DMHY Crawler Latency",
      [
        {
          expr: "histogram_quantile(0.95, sum by (le) (rate(takuhai_dmhy_crawl_duration_seconds_bucket[$__rate_interval])))",
          legendFormat: "crawl p95",
        },
        {
          expr: "histogram_quantile(0.95, sum by (le) (rate(takuhai_dmhy_fetch_duration_seconds_bucket[$__rate_interval])))",
          legendFormat: "fetch p95",
        },
      ],
      0,
      76,
      "s",
    ),
  ]);
}

function dashboard(uid: string, folder: string, title: string, panels: Record<string, unknown>[]): DashboardSpec {
  return {
    uid,
    folder,
    definition: {
      editable: true,
      refresh: "30s",
      schemaVersion: 39,
      tags: ["k2"],
      time: { from: "now-6h", to: "now" },
      timezone: "browser",
      title,
      uid,
      panels,
    },
  };
}

function stat(id: number, title: string, expr: string, x: number, y: number, unit = "short") {
  return panel(id, title, "stat", x, y, 6, 4, [{ expr }], unit, {
    colorMode: "value",
    graphMode: "area",
    justifyMode: "auto",
    reduceOptions: { calcs: ["lastNotNull"], fields: "", values: false },
  });
}

function timeSeries(id: number, title: string, targets: PanelTarget[], x: number, y: number, unit = "short") {
  return panel(id, title, "timeseries", x, y, 24, 8, targets, unit, {
    legend: { calcs: ["lastNotNull"], displayMode: "table", placement: "right", showLegend: true },
    tooltip: { mode: "single" },
  });
}

function panel(
  id: number,
  title: string,
  type: string,
  x: number,
  y: number,
  w: number,
  h: number,
  targets: PanelTarget[],
  unit: string,
  options: Record<string, unknown>,
) {
  return {
    datasource: DATASOURCE,
    fieldConfig: { defaults: { unit }, overrides: [] },
    gridPos: { h, w, x, y },
    id,
    options,
    targets: targets.map((target, index) => ({
      datasource: DATASOURCE,
      expr: target.expr,
      legendFormat: target.legendFormat,
      refId: String.fromCharCode(65 + index),
    })),
    title,
    type,
  };
}
