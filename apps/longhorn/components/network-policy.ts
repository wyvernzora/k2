import type { Construct } from "constructs";

import { K2Chart, Namespace, NfsContext } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, egress, endpoint, ingress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { endpoints } from "../index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;
    const nas = NfsContext.of(this).server;

    new AllowPomeriumToBackend(this, "pomerium-to-longhorn-ui", {
      ...endpoints.http(),
    });
    new EndpointNetworkPolicy(this, "longhorn-cluster-access", {
      endpoint: endpoint(namespace, {}, "longhorn"),
      ingress: [...ingress.fromCluster(), ...ingress.fromNodes()],
      egress: [
        { to: { entity: "cluster" } },
        { to: { entity: "host" } },
        { to: { entity: "remote-node" } },
        ...egress.toKubeApiServer(),
        ...egress.toDns(),
        ...egress.toCidrs([`${nas}/32`]),
      ],
    });
    new PrometheusPodScrape(this, "longhorn-manager-metrics", {
      target: endpoint(namespace, { app: "longhorn-manager" }, "longhorn-manager"),
      ports: [tcp(9500)],
    });
  }
}
