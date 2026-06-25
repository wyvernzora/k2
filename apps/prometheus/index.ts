import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { Prometheus } from "./components/prometheus/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * as crd from "./lib/crd.js";
export * from "./lib/pod-scrape.js";

const PROMETHEUS_NAMESPACE = "prometheus";
const GRAFANA_HTTP_PORT = 3000;
const GRAFANA_LABELS = {
  "app.kubernetes.io/name": "grafana",
  "app.kubernetes.io/instance": "prometheus",
};

export const endpoints = {
  grafanaHttp(): BackendTarget {
    return { backend: grafanaEndpoint(), ports: [tcp(GRAFANA_HTTP_PORT)] };
  },
};

function grafanaEndpoint(): PolicyEndpoint {
  return endpoint(PROMETHEUS_NAMESPACE, GRAFANA_LABELS, "grafana");
}

export const createAppResources: AppResourceFunc = app => {
  new Prometheus(app, "prometheus");
  new NetworkPolicy(app, "network-policy");
};
