import { ApiObject, JsonPatch } from "cdk8s";
import { Pods, Protocol, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { POMERIUM_LABELS, POMERIUM_PROXY_SERVICE_NAME } from "../../lib/constants.js";

import { metadata } from "./metadata.js";

const METRICS_SERVICE_NAME = "pomerium-metrics";

export function createServices(scope: Construct): void {
  const metrics = new Service(scope, "metrics-service", metricsService());
  ApiObject.of(metrics).addJsonPatch(JsonPatch.remove("/spec/selector"));

  const proxy = new Service(scope, "proxy-service", proxyService(scope));
  ApiObject.of(proxy).addJsonPatch(JsonPatch.add("/spec/ipFamilyPolicy", "PreferDualStack"));
}

function metricsService() {
  return {
    metadata: metadata(METRICS_SERVICE_NAME),
    type: ServiceType.CLUSTER_IP,
    ports: [servicePort("metrics", 9090, 9090)],
  };
}

function proxyService(scope: Construct) {
  const proxyMetadata = metadata(POMERIUM_PROXY_SERVICE_NAME);
  return {
    metadata: {
      ...proxyMetadata,
      labels: { ...proxyMetadata.labels, ...POMERIUM_LABELS },
    },
    type: ServiceType.LOAD_BALANCER,
    selector: Pods.select(scope, "pomerium-proxy-service-pods", { labels: POMERIUM_LABELS }),
    ports: [
      servicePort("https", 443, 8443),
      servicePort("quic", 443, 443, Protocol.UDP),
      servicePort("http", 80, 8080),
    ],
  };
}

function servicePort(name: string, port: number, targetPort: number, protocol = Protocol.TCP) {
  return {
    name,
    port,
    protocol,
    targetPort,
  };
}
