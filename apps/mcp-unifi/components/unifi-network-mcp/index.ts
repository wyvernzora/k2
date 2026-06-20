import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";
import { AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { UNIFI_NETWORK_MCP_SERVICE_NAME } from "../../constants.js";

import { UnifiNetworkMcpDeployment } from "./deployment.js";
import { UnifiNetworkMcpService } from "./service.js";

const UNIFI_MCP_HOST_PREFIX = "unifi-mcp";
const UNIFI_MCP_SECRET_NAME = "unifi-network-mcp";
const UNIFI_MCP_SECRET_ID = "ouq44qyvowkyhuyw7waubowa3a";

export class UnifiNetworkMcp extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    new ManagedSecret(this, "credentials", {
      metadata: { name: UNIFI_MCP_SECRET_NAME },
      secretId: UNIFI_MCP_SECRET_ID,
      fields: {
        "api-key": "credential",
      },
    });

    new UnifiNetworkMcpDeployment(this, "deployment", {
      credentialsSecretName: UNIFI_MCP_SECRET_NAME,
      host: ApexDomain.of(this).subdomain("unifi"),
    });
    new UnifiNetworkMcpService(this, "service");
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host: ApexDomain.of(this).subdomain(UNIFI_MCP_HOST_PREFIX),
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: UNIFI_NETWORK_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
