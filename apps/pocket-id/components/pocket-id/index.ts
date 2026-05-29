import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { PublicIngress } from "@k2/pomerium";

import { POCKET_ID_HOST_PREFIX, POCKET_ID_SERVICE_NAME } from "../../lib/constants.js";

import { PocketIdDatabase } from "./database.js";
import { PocketIdDeployment } from "./deployment.js";
import { PocketIdSecret } from "./secret.js";
import { PocketIdService } from "./service.js";
import { PocketIdSetup } from "./setup.js";

export class PocketId extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const appUrl = `https://${ApexDomain.of(this).subdomain(POCKET_ID_HOST_PREFIX)}`;
    const secret = new PocketIdSecret(this, "secret");
    const database = new PocketIdDatabase(this, "database");

    new PocketIdDeployment(this, "deployment", {
      appUrl,
      credentialsSecretName: database.credentialsSecretName,
      secretName: secret.secretName,
    });
    new PocketIdService(this, "service");
    new PocketIdSetup(this, "setup");
    new PublicIngress(this, "ingress", {
      host: new URL(appUrl).host,
      serviceName: POCKET_ID_SERVICE_NAME,
      servicePort: "http",
    });
  }
}
