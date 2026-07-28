import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";
import { AuthenticatedIngress, AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { KURA_SERVICE_NAME } from "../../constants.js";

import { LibraryManagerConfig } from "./config.js";
import { KuraDeployment } from "./deployment.js";
import { KuraGatewayDeployment } from "./gateway-deployment.js";
import { KuraService, LibraryManagerService } from "./service.js";

const KURA_HOST_PREFIX = "kura";
const TVDB_SECRET_NAME = "kura-tvdb";
const TVDB_SECRET_ID = "q4xf32di7npmvc7e62amvgd574";

export class Kura extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const host = ApexDomain.of(this).subdomain(KURA_HOST_PREFIX);

    new ManagedSecret(this, "tvdb-secret", {
      metadata: { name: TVDB_SECRET_NAME },
      secretId: TVDB_SECRET_ID,
      fields: { credential: "credential" },
    });
    const config = new LibraryManagerConfig(this, "library-manager-config");
    new KuraDeployment(this, "deployment", {
      configChecksum: config.checksum,
      configName: config.name,
      tvdbSecretName: TVDB_SECRET_NAME,
      volumes: {
        anime: K2Volume.mountNfs({ path: "/mnt/data/media/anime" }),
      },
    });
    new LibraryManagerService(this, "library-manager-service");
    new KuraGatewayDeployment(this, "gateway-deployment");
    new KuraService(this, "service");
    new AuthenticatedIngress(this, "ingress", {
      host,
      serviceName: KURA_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host,
      path: "/mcp/v1",
      mcpPath: "/mcp/v1",
      serviceName: KURA_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
