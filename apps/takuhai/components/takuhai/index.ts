import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { TAKUHAI_MCP_SERVICE_NAME } from "../../constants.js";

import { TakuhaiDatabase } from "./database.js";
import { TakuhaiCrawlerDeployment, TakuhaiDeployment } from "./deployment.js";
import { TakuhaiCrawlerService, TakuhaiMcpService, TakuhaiService } from "./service.js";

const TAKUHAI_HOST_PREFIX = "takuhai";

export class Takuhai extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const database = new TakuhaiDatabase(this, "database");

    new TakuhaiDeployment(this, "deployment", {
      credentialsSecretName: database.credentialsSecretName,
    });
    new TakuhaiCrawlerDeployment(this, "crawler-deployment");
    new TakuhaiService(this, "service");
    new TakuhaiMcpService(this, "mcp-service");
    new TakuhaiCrawlerService(this, "crawler-service");
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host: ApexDomain.of(this).subdomain(TAKUHAI_HOST_PREFIX),
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: TAKUHAI_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
