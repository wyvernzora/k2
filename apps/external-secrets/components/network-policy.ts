import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, fqdn, ingress, tcp } from "@k2/cilium";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "controller-onepassword-egress", {
      endpoint: endpoint(
        namespace,
        {
          "app.kubernetes.io/instance": "external-secrets",
          "app.kubernetes.io/name": "external-secrets",
        },
        "external-secrets-controller",
      ),
      egress: [
        ...egress.toDns(),
        ...egress.toFqdns([
          fqdn.name("1password.com"),
          fqdn.pattern("**.1password.com"),
          fqdn.pattern("**.1passwordservices.com"),
        ]),
      ],
    });
    new EndpointNetworkPolicy(this, "webhook-admission-ingress", {
      endpoint: endpoint(
        namespace,
        {
          "app.kubernetes.io/instance": "external-secrets",
          "app.kubernetes.io/name": "external-secrets-webhook",
        },
        "external-secrets-webhook",
      ),
      ingress: ingress.fromNodes(tcp(10250)),
    });
  }
}
