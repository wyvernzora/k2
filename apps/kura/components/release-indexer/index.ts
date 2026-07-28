import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";

import { ReleaseIndexerConfig } from "./config.js";
import { ReleaseIndexerDatabase } from "./database.js";
import { ReleaseIndexerDeployment } from "./deployment.js";
import { ReleaseIndexerService } from "./service.js";

export class ReleaseIndexer extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const database = new ReleaseIndexerDatabase(this, "database");
    const config = new ReleaseIndexerConfig(this, "config");
    new ReleaseIndexerDeployment(this, "deployment", {
      configChecksum: config.checksum,
      configName: config.name,
      credentialsSecretName: database.credentialsSecretName,
    });
    new ReleaseIndexerService(this, "service");
  }
}
