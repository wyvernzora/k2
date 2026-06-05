import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { HOMER_SERVICE_NAME } from "../../constants.js";

import { HomerConfig } from "./config.js";
import { HomerDeployment } from "./deployment.js";
import { HomerService } from "./service.js";

const HOMER_HOST_PREFIX = "home";

export class Homer extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const config = new HomerConfig(this, "config");
    new HomerDeployment(this, "deployment", {
      configName: config.name,
      configChecksum: config.checksum,
    });
    new HomerService(this, "service");
    new AuthenticatedIngress(this, "ingress", {
      host: ApexDomain.of(this).subdomain(HOMER_HOST_PREFIX),
      serviceName: HOMER_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
