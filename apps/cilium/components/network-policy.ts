import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { AllowPomeriumToBackend } from "@k2/pomerium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { endpoints } from "../index.js";
import { endpoint, tcp } from "../lib/netpol/index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new AllowPomeriumToBackend(this, "pomerium-to-hubble-ui", {
      ...endpoints.hubbleUiHttp(),
    });
    new PrometheusPodScrape(this, "cilium-operator-metrics", {
      target: endpoint(namespace, { "io.cilium/app": "operator" }, "cilium-operator"),
      ports: [tcp(9963)],
    });
    new PrometheusPodScrape(this, "cilium-envoy-metrics", {
      target: endpoint(namespace, { "k8s-app": "cilium-envoy" }, "cilium-envoy"),
      ports: [tcp(9964)],
    });
  }
}
