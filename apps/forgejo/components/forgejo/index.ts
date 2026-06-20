import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { K2Chart, K2Volume } from "@k2/cdk-lib";

import { ForgejoDatabase } from "./database.js";
import { FORGEJO_APPDATA_CLAIM_NAME, ForgejoDeployment } from "./deployment.js";
import { ForgejoSecret } from "./secret.js";
import { ForgejoService } from "./service.js";
import { ForgejoSetup } from "./setup.js";

export class Forgejo extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const secret = new ForgejoSecret(this, "secret");
    const database = new ForgejoDatabase(this, "database");

    new ForgejoDeployment(this, "deployment", {
      credentialsSecretName: database.credentialsSecretName,
      secretName: secret.secretName,
      volumes: {
        appdata: K2Volume.replicated({ name: FORGEJO_APPDATA_CLAIM_NAME, size: Size.gibibytes(20) }),
      },
    });
    new ForgejoService(this, "service");
    new ForgejoSetup(this, "setup", {
      appdataClaimName: FORGEJO_APPDATA_CLAIM_NAME,
      credentialsSecretName: database.credentialsSecretName,
      secretName: secret.secretName,
    });
  }
}
