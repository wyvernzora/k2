import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, endpoint, ingress, tcp } from "@k2/cilium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { NEXUS_CLUSTER_NAME } from "../lib/nexus.js";

export class NetworkPolicies extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "cnpg-webhook-admission-ingress", {
      endpoint: endpoint(
        namespace,
        {
          "app.kubernetes.io/instance": "cnpg",
          "app.kubernetes.io/name": "cloudnative-pg",
        },
        "cnpg-webhook",
      ),
      ingress: ingress.fromNodes(tcp(9443)),
    });
    new PrometheusPodScrape(this, "nexus-postgresql-metrics", {
      target: endpoint(
        namespace,
        {
          "cnpg.io/cluster": NEXUS_CLUSTER_NAME,
          "cnpg.io/podRole": "instance",
        },
        "nexus-postgresql",
      ),
      ports: [tcp(9187)],
    });
    new PrometheusPodScrape(this, "dbclaim-operator-metrics", {
      target: endpoint(
        namespace,
        {
          "app.kubernetes.io/instance": "dbclaim-operator",
          "app.kubernetes.io/name": "dbclaim-operator",
        },
        "dbclaim-operator",
      ),
      ports: [tcp(8080)],
    });
  }
}
