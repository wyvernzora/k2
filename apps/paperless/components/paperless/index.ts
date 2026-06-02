import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { PaperlessDatabase } from "./database.js";
import { PaperlessDeployment } from "./deployment.js";
import { PAPERLESS_SERVICE_NAME } from "./labels.js";
import { RedisDeployment } from "./redis.js";
import { PaperlessSecret } from "./secret.js";
import { PaperlessService, RedisService } from "./service.js";

const PAPERLESS_HOST_PREFIX = "paperless";
const DOCUMENTS_PATH = "/mnt/data/documents/archive";

export class Paperless extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const host = ApexDomain.of(this).subdomain(PAPERLESS_HOST_PREFIX);
    const appUrl = `https://${host}`;
    const secret = new PaperlessSecret(this, "secret");
    const database = new PaperlessDatabase(this, "database");

    new RedisDeployment(this, "redis", { secretName: secret.secretName });
    new RedisService(this, "redis-service");
    new PaperlessDeployment(this, "deployment", {
      appUrl,
      credentialsSecretName: database.credentialsSecretName,
      secretName: secret.secretName,
      volumes: {
        data: K2Volume.replicated({ name: "paperless-data", size: Size.gibibytes(4) }),
        documents: K2Volume.mountNfs({ path: DOCUMENTS_PATH }),
      },
    });
    new PaperlessService(this, "service");
    new AuthenticatedIngress(this, "ingress", {
      host,
      serviceName: PAPERLESS_SERVICE_NAME,
      servicePort: "http",
      passIdentityHeaders: true,
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
