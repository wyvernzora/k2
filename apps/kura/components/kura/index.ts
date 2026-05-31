import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";
import { AuthenticatedIngress, AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { KuraDeployment } from "./deployment.js";
import { KURA_MCP_SERVICE_NAME, KURA_SERVICE_NAME } from "./labels.js";
import { KuraMcpService, KuraService } from "./service.js";

const KURA_HOST_PREFIX = "kura";
const TVDB_SECRET_NAME = "kura-tvdb";

export class Kura extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const host = ApexDomain.of(this).subdomain(KURA_HOST_PREFIX);

    new ManagedSecret(this, "tvdb-secret", {
      metadata: { name: TVDB_SECRET_NAME },
      secret: "TheTVDB v4 API",
      fields: { credential: "credential" },
    });
    new KuraDeployment(this, "deployment", {
      tvdbSecretName: TVDB_SECRET_NAME,
      volumes: {
        anime: K2Volume.nfs({ path: "/mnt/data/media/anime" }),
      },
    });
    new KuraService(this, "service");
    new KuraMcpService(this, "mcp-service");
    new AuthenticatedIngress(this, "ingress", {
      host,
      serviceName: KURA_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host,
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: KURA_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
