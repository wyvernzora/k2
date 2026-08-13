import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { AuthenticatedIngress, PublicIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { N8NDatabase } from "./database.js";
import { N8NDeployment } from "./deployment.js";
import { N8N_SERVICE_NAME } from "./labels.js";
import { N8NSecret } from "./secret.js";
import { N8NService } from "./service.js";

const N8N_HOST_PREFIX = "n8n";
// Native MCP clients need unauthenticated discovery and token endpoints. The
// consent UI remains on /oauth/consent behind the authenticated catch-all route.
const N8N_MCP_PUBLIC_ROUTES = [
  ["mcp-server-ingress", "/mcp-server/http"],
  ["mcp-oauth-ingress", "/mcp-oauth"],
  ["mcp-authorization-metadata-ingress", "/.well-known/oauth-authorization-server"],
  ["mcp-resource-metadata-ingress", "/.well-known/oauth-protected-resource/mcp-server/http"],
] as const;

export class N8N extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const host = ApexDomain.of(this).subdomain(N8N_HOST_PREFIX);
    const secret = new N8NSecret(this, "secret");
    const database = new N8NDatabase(this, "database");

    new N8NDeployment(this, "deployment", {
      appUrl: `https://${host}/`,
      credentialsSecretName: database.credentialsSecretName,
      secretName: secret.secretName,
      userManagementSecretName: secret.userManagementSecretName,
      volumes: {
        appdata: K2Volume.iscsi({ size: Size.gibibytes(4) }),
        opencodeHome: K2Volume.iscsi({ size: Size.gibibytes(8) }),
        codexHome: K2Volume.iscsi({ size: Size.gibibytes(8) }),
      },
    });
    new N8NService(this, "service");
    new AuthenticatedIngress(this, "ingress", {
      host,
      serviceName: N8N_SERVICE_NAME,
      servicePort: "http",
      allowWebsockets: true,
      passIdentityHeaders: true,
      policy: authenticatedSourceIpPolicy(),
      preserveHostHeader: true,
    });
    for (const [id, path] of N8N_MCP_PUBLIC_ROUTES) {
      new PublicIngress(this, id, {
        host,
        serviceName: N8N_SERVICE_NAME,
        servicePort: "http",
        path,
        preserveHostHeader: true,
      });
    }
  }
}
