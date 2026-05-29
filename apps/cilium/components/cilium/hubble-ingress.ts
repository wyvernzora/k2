import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

const HUBBLE_HOST_PREFIX = "hubble";
const HUBBLE_UI_SERVICE_NAME = "hubble-ui";

export class HubbleIngress extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AuthenticatedIngress(this, "ingress", {
      host: ApexDomain.of(this).subdomain(HUBBLE_HOST_PREFIX),
      serviceName: HUBBLE_UI_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
