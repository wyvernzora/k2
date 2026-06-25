import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";
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
  }
}
