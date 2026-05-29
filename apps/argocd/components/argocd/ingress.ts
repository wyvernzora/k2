import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

const ARGOCD_HOST_PREFIX = "argo";
const ARGOCD_SERVER_SERVICE_NAME = "argocd-server";

export class ArgoCDIngress extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AuthenticatedIngress(this, "ingress", {
      host: ApexDomain.of(this).subdomain(ARGOCD_HOST_PREFIX),
      serviceName: ARGOCD_SERVER_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
