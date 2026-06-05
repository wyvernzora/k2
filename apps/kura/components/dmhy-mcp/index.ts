import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { DMHY_MCP_SERVICE_NAME } from "../../constants.js";

import { DmhyMcpDeployment } from "./deployment.js";
import { DmhyMcpService } from "./service.js";

const DMHY_HOST_PREFIX = "dmhy";

export class DmhyMcp extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const host = ApexDomain.of(this).subdomain(DMHY_HOST_PREFIX);

    new DmhyMcpDeployment(this, "deployment");
    new DmhyMcpService(this, "service");
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host,
      name: "dmhy-mcp-ingress",
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: DMHY_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
