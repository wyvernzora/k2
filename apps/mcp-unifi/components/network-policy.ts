import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

const UNIFI_CONTROLLER_CIDR = "10.10.1.1/32";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const unifiNetworkMcp = endpoints.http();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-unifi-network-mcp", {
      ...unifiNetworkMcp,
    });
    new EndpointNetworkPolicy(this, "unifi-network-mcp-egress", {
      endpoint: unifiNetworkMcp.backend,
      egress: [...egress.toCidrs([UNIFI_CONTROLLER_CIDR], tcp(443))],
    });
  }
}
