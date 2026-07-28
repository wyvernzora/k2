import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";

import { TakuhaiDatabase } from "./database.js";
import { TakuhaiCrawlerDeployment, TakuhaiDeployment, TakuhaiNyaaCrawlerDeployment } from "./deployment.js";
import { TakuhaiCrawlerService, TakuhaiNyaaCrawlerService, TakuhaiService } from "./service.js";

export class Takuhai extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const database = new TakuhaiDatabase(this, "database");

    new TakuhaiDeployment(this, "deployment", {
      credentialsSecretName: database.credentialsSecretName,
    });
    new TakuhaiCrawlerDeployment(this, "crawler-deployment");
    new TakuhaiNyaaCrawlerDeployment(this, "crawler-nyaa-deployment");
    new TakuhaiService(this, "service");
    new TakuhaiCrawlerService(this, "crawler-service");
    new TakuhaiNyaaCrawlerService(this, "crawler-nyaa-service");
  }
}
