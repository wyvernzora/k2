import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, tcp, type PolicyEndpoint, type PortSpec } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { endpoints } from "../index.js";

const ARGOCD_METRICS_TARGETS: MetricsTarget[] = [
  {
    name: "argocd-application-controller",
    labels: {
      "app.kubernetes.io/instance": "argocd",
      "app.kubernetes.io/name": "argocd-application-controller",
    },
    ports: [tcp(8082)],
  },
  {
    name: "argocd-applicationset-controller",
    labels: {
      "app.kubernetes.io/instance": "argocd",
      "app.kubernetes.io/name": "argocd-applicationset-controller",
    },
    ports: [tcp(8080)],
  },
  {
    name: "argocd-repo-server",
    labels: {
      "app.kubernetes.io/instance": "argocd",
      "app.kubernetes.io/name": "argocd-repo-server",
    },
    ports: [tcp(8084)],
  },
  {
    name: "argocd-server",
    labels: {
      "app.kubernetes.io/instance": "argocd",
      "app.kubernetes.io/name": "argocd-server",
    },
    ports: [tcp(8083)],
  },
];

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new AllowPomeriumToBackend(this, "pomerium-to-argocd-server", {
      ...endpoints.http(),
    });
    for (const target of ARGOCD_METRICS_TARGETS) {
      const targetEndpoint = metricsEndpoint(namespace, target);
      new PrometheusPodScrape(this, `${target.name}-metrics`, {
        target: targetEndpoint,
        ports: target.ports,
      });
    }
  }
}

interface MetricsTarget {
  readonly name: string;
  readonly labels: Record<string, string>;
  readonly ports: PortSpec[];
}

function metricsEndpoint(namespace: string, target: MetricsTarget): PolicyEndpoint {
  return endpoint(namespace, target.labels, target.name);
}
