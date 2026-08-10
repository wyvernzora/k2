import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, egress, endpoint, tcp } from "@k2/cilium";

import { APPLIANCE_FABRIC_ADDRESS, APPLIANCE_SSH_PORT } from "../constants.js";

/**
 * The controller drives the appliance's zvols and LIO config over SSH on the
 * storage fabric. Node-plugin iSCSI traffic is kernel/host-level and never
 * crosses the CNI, so no policy covers it.
 */
export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;

    new EndpointNetworkPolicy(this, "democratic-csi-access", {
      endpoint: endpoint(namespace, {}, "democratic-csi"),
      egress: [
        ...egress.toKubeApiServer(),
        ...egress.toDns(),
        ...egress.toCidrs([`${APPLIANCE_FABRIC_ADDRESS}/32`], tcp(APPLIANCE_SSH_PORT)),
      ],
    });
  }
}
