import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, endpoint, ingress, tcp } from "@k2/cilium";

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
  }
}
