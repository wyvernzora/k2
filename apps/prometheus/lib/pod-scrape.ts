import type { Construct } from "constructs";

import { endpoint, EndpointNetworkPolicy, type PolicyEndpoint, type PortSpec } from "@k2/cilium";

import { PodMonitor, type PodMonitorSpecPodMetricsEndpoints } from "../crds/monitoring.coreos.com.js";

const PROMETHEUS_NAMESPACE = "prometheus";
const PROMETHEUS_LABELS = {
  "app.kubernetes.io/name": "prometheus",
  "operator.prometheus.io/name": "prometheus-kube-prometheus-prometheus",
};

export interface PrometheusPodScrapeProps {
  readonly target: PolicyEndpoint;
  readonly ports: PortSpec[];
  readonly path?: string;
  readonly interval?: string;
}

export class PrometheusPodScrape extends PodMonitor {
  public constructor(scope: Construct, id: string, props: PrometheusPodScrapeProps) {
    super(scope, id, {
      metadata: { name: id },
      spec: {
        namespaceSelector: { matchNames: [props.target.namespace] },
        selector: { matchLabels: props.target.labels },
        podMetricsEndpoints: props.ports.map(port => podMetricsEndpoint(port, props)),
      },
    });

    new EndpointNetworkPolicy(scope, `${id}-network`, {
      endpoint: prometheusEndpoint(),
      egress: [{ to: { endpoint: props.target }, ports: props.ports }],
    });
    new EndpointNetworkPolicy(scope, `${id}-ingress`, {
      endpoint: props.target,
      ingress: [{ from: { endpoint: prometheusEndpoint() }, ports: props.ports }],
    });
  }
}

function podMetricsEndpoint(port: PortSpec, props: PrometheusPodScrapeProps): PodMonitorSpecPodMetricsEndpoints {
  if (port.protocol !== "TCP") {
    throw new Error("PrometheusPodScrape only supports TCP metrics ports");
  }
  return {
    path: props.path,
    interval: props.interval,
    ...(typeof port.port === "number" ? { portNumber: port.port } : { port: port.port }),
  };
}

function prometheusEndpoint(): PolicyEndpoint {
  return endpoint(PROMETHEUS_NAMESPACE, PROMETHEUS_LABELS, "prometheus");
}
