import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { endpoints } from "../index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new AllowPomeriumToBackend(this, "pomerium-to-longhorn-ui", {
      ...endpoints.http(),
    });
    new PrometheusPodScrape(this, "longhorn-manager-metrics", {
      target: endpoint(namespace, { app: "longhorn-manager" }, "longhorn-manager"),
      ports: [tcp(9500)],
    });
  }
}
