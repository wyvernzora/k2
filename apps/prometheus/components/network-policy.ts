import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, egress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints, workloads } from "../index.js";
import { PrometheusPodScrape } from "../lib/pod-scrape.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const grafana = endpoints.grafanaHttp();

    new AllowPomeriumToBackend(this, "pomerium-to-grafana", grafana);
    new PrometheusPodScrape(this, "grafana-metrics", {
      target: grafana.backend,
      ports: grafana.ports,
    });
    // Selecting the Prometheus server with any egress rule puts it into Cilium
    // default-deny egress, so the whole scrape baseline (in-cluster targets,
    // node-level exporters, apiserver discovery, DNS) has to be listed
    // alongside the appliance CIDR or monitoring goes dark cluster-wide.
    new EndpointNetworkPolicy(this, "prometheus-egress", {
      endpoint: workloads.prometheus(),
      egress: [
        { to: { entity: "cluster" } },
        { to: { entity: "host" } },
        { to: { entity: "remote-node" } },
        ...egress.toKubeApiServer(),
        ...egress.toDns(),
        ...egress.toCidrs(["10.10.9.250/32"], tcp(9100)),
      ],
    });
  }
}
