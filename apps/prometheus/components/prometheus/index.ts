import type { Construct } from "constructs";

import { ApexDomain, HelmCharts, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { ScrapeConfig, type ScrapeConfigProps } from "../../crds/monitoring.coreos.com.js";
import { DASHBOARD_FOLDER_ANNOTATION, DASHBOARD_ROOT, GrafanaDashboards } from "../dashboards/index.js";

import { GRAFANA_ADMIN_SECRET_NAME, GrafanaAdminSecret } from "./admin-secret.js";

const GRAFANA_HOST_PREFIX = "grafana";
const GRAFANA_SERVICE_NAME = "prometheus-grafana";

export class Prometheus extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const grafanaHost = ApexDomain.of(this).subdomain(GRAFANA_HOST_PREFIX);

    new GrafanaAdminSecret(this, "grafana-admin-secret");
    new GrafanaDashboards(this, "grafana-dashboards");
    new ScrapeConfig(this, "storage-appliance-scrape", storageApplianceScrapeConfig());
    HelmCharts.of(this).asChart(this, "prometheus", "kube-prometheus-stack", prometheusValues(grafanaHost));
    new AuthenticatedIngress(this, "grafana-ingress", {
      host: grafanaHost,
      serviceName: GRAFANA_SERVICE_NAME,
      servicePort: 80,
      passIdentityHeaders: true,
      policy: authenticatedSourceIpPolicy(),
    });
  }
}

function prometheusValues(grafanaHost: string) {
  return {
    crds: { enabled: false },
    defaultRules: {
      rules: {
        kubeControllerManager: false,
        kubeProxy: false,
        kubeSchedulerAlerting: false,
        kubeSchedulerRecording: false,
      },
    },
    grafana: grafanaValues(grafanaHost),
    kubeControllerManager: { enabled: false },
    kubeProxy: { enabled: false },
    kubeScheduler: { enabled: false },
    prometheus: { prometheusSpec: prometheusSpec() },
    "prometheus-node-exporter": nodeExporterValues(),
  };
}

function nodeExporterValues() {
  return {
    extraArgs: nodeExporterExtraArgs(),
    extraHostVolumeMounts: [nodeExporterTextfileMount()],
    prometheus: { monitor: { relabelings: nodeExporterRelabelings() } },
  };
}

function nodeExporterTextfileMount() {
  return {
    name: "textfile",
    hostPath: "/var/lib/prometheus/node-exporter",
    mountPath: "/var/lib/prometheus/node-exporter",
    type: "DirectoryOrCreate",
    readOnly: true,
  };
}

// Without this, k8s nodes identify as "10.10.9.11:9100" while the storage
// appliance — scraped by a static config — identifies as "k2-st-0e12". Any
// fleet-wide panel or query then mixes IP:port rows with hostname rows for
// the same k2_node_* metrics. Relabel at the scrape so `instance` is the node
// name everywhere and queries can stay `by (instance)`.
//
// Same Helm caveat as extraArgs above: lists are replaced, not merged, so
// anything the chart ships here has to be repeated. It currently ships none —
// no target carries kubernetes_node today.
function nodeExporterRelabelings() {
  return [
    {
      action: "replace",
      sourceLabels: ["__meta_kubernetes_pod_node_name"],
      targetLabel: "instance",
    },
  ];
}

// Helm replaces lists instead of merging them, so the chart's own filesystem
// excludes have to be repeated here or every kubelet/containerd/overlay mount
// gets node_filesystem_* series (and NodeFilesystem* alerts) on all 7 nodes.
function nodeExporterExtraArgs() {
  return [
    "--collector.filesystem.mount-points-exclude=^/(dev|proc|sys|run/containerd/.+|var/lib/docker/.+|var/lib/kubelet/.+)($|/)",
    "--collector.filesystem.fs-types-exclude=^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs|erofs)$",
    "--collector.textfile.directory=/var/lib/prometheus/node-exporter",
  ];
}

function grafanaValues(grafanaHost: string) {
  return {
    admin: {
      existingSecret: GRAFANA_ADMIN_SECRET_NAME,
      userKey: "admin-user",
      passwordKey: "admin-password",
    },
    serviceMonitor: { enabled: false },
    "grafana.ini": {
      "auth.jwt": {
        enabled: true,
        header_name: "X-Pomerium-Jwt-Assertion",
        email_claim: "email",
        username_claim: "email",
        jwk_set_url: `https://${grafanaHost}/.well-known/pomerium/jwks.json`,
        auto_sign_up: true,
        role_attribute_path: "email == 'wyvernzora@gmail.com' && 'GrafanaAdmin' || 'Viewer'",
        allow_assign_grafana_admin: true,
        cache_ttl: "60m",
      },
    },
    sidecar: { dashboards: grafanaDashboardSidecarValues() },
    ingress: { enabled: false },
  };
}

function grafanaDashboardSidecarValues() {
  return {
    annotations: {
      [DASHBOARD_FOLDER_ANNOTATION]: `${DASHBOARD_ROOT}/Kubernetes`,
    },
    folderAnnotation: DASHBOARD_FOLDER_ANNOTATION,
    provider: {
      allowUiUpdates: true,
      foldersFromFilesStructure: true,
    },
  };
}

function prometheusSpec() {
  return {
    retention: "15d",
    // Leave headroom for the WAL, m-mapped head chunks, and compaction. This
    // is 81.25% of the 32Gi PVC, within Prometheus's recommended 80-85%.
    retentionSize: "26GiB",
    ruleSelectorNilUsesHelmValues: false,
    ruleSelector: {},
    ruleNamespaceSelector: {},
    serviceMonitorSelectorNilUsesHelmValues: false,
    serviceMonitorSelector: {},
    serviceMonitorNamespaceSelector: {},
    podMonitorSelectorNilUsesHelmValues: false,
    podMonitorSelector: {},
    podMonitorNamespaceSelector: {},
    probeSelectorNilUsesHelmValues: false,
    probeSelector: {},
    probeNamespaceSelector: {},
    scrapeConfigSelectorNilUsesHelmValues: false,
    scrapeConfigSelector: {},
    scrapeConfigNamespaceSelector: {},
    storageSpec: prometheusStorageSpec(),
  };
}

function storageApplianceScrapeConfig(): ScrapeConfigProps {
  return {
    metadata: { name: "storage-appliance" },
    spec: {
      jobName: "storage-appliance",
      staticConfigs: [storageApplianceTarget()],
    },
  };
}

// The appliance lives outside the cluster, so it is a static target rather
// than a discovered one. Labelled by node name so dashboards never match on
// the raw address.
function storageApplianceTarget() {
  return {
    labels: { instance: "k2-st-0e12", role: "storage" },
    targets: ["10.10.9.250:9100"],
  };
}

function prometheusStorageSpec() {
  return {
    volumeClaimTemplate: {
      spec: prometheusStorageClaimSpec(),
    },
  };
}

function prometheusStorageClaimSpec() {
  return {
    storageClassName: "k2-iscsi",
    accessModes: ["ReadWriteOnce"],
    resources: { requests: storageRequests() },
  };
}

function storageRequests() {
  return {
    storage: "32Gi",
  };
}
