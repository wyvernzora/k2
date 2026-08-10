import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, egress, endpoint } from "@k2/cilium";

/** The controller only ever talks to the Kubernetes API. */
export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new EndpointNetworkPolicy(this, "snapshot-controller-access", {
      endpoint: endpoint(namespace, {}, "snapshot-controller"),
      egress: [...egress.toKubeApiServer(), ...egress.toDns()],
    });
  }
}
