import { IntOrString, KubeService } from "cdk8s-plus-32/lib/imports/k8s.js";
import type { Construct } from "constructs";

import { POMERIUM_LABELS, POMERIUM_PROXY_SERVICE_NAME } from "../../lib/constants.js";

import { metadata } from "./metadata.js";

const METRICS_SERVICE_NAME = "pomerium-metrics";

export function createServices(scope: Construct): void {
  new KubeService(scope, "metrics-service", metricsService());
  new KubeService(scope, "proxy-service", proxyService());
}

function metricsService() {
  return {
    metadata: metadata(METRICS_SERVICE_NAME),
    spec: {
      type: "ClusterIP",
      ports: [servicePort("metrics", 9090, "metrics")],
    },
  };
}

function proxyService() {
  const proxyMetadata = metadata(POMERIUM_PROXY_SERVICE_NAME);
  return {
    metadata: {
      ...proxyMetadata,
      labels: { ...proxyMetadata.labels, ...POMERIUM_LABELS },
    },
    spec: {
      type: "LoadBalancer",
      ipFamilyPolicy: "PreferDualStack",
      selector: POMERIUM_LABELS,
      ports: [
        servicePort("https", 443, "https"),
        servicePort("quic", 443, "quic", "UDP"),
        servicePort("http", 80, "http"),
      ],
    },
  };
}

function servicePort(name: string, port: number, targetPort: string, protocol = "TCP") {
  return {
    name,
    port,
    protocol,
    targetPort: IntOrString.fromString(targetPort),
  };
}
