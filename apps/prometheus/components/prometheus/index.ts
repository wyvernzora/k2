import type { Construct } from "constructs";

import { ApexDomain, HelmCharts, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

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
    grafana: grafanaValues(grafanaHost),
    prometheus: { prometheusSpec: prometheusSpec() },
  };
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
      allowUiUpdates: false,
      foldersFromFilesStructure: true,
    },
  };
}

function prometheusSpec() {
  return {
    retention: "15d",
    retentionSize: "15GiB",
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
    storageSpec: prometheusStorageSpec(),
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
    storageClassName: "longhorn",
    accessModes: ["ReadWriteOnce"],
    resources: { requests: storageRequests() },
  };
}

function storageRequests() {
  return {
    storage: "20Gi",
  };
}
