import type { Construct } from "constructs";

import { CiliumNetworkPolicy } from "../../crds/cilium.io.js";

export class DefaultDenyNetworkPolicy extends CiliumNetworkPolicy {
  public constructor(scope: Construct, id = "default-deny") {
    super(scope, id, {
      metadata: {
        name: "default-deny",
      },
      spec: {
        endpointSelector: {},
        ingress: [],
        egress: [],
      },
    });
  }
}
