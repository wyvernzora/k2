import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import {
  AuthenticatedIngress,
  AuthenticatedMcpIngress,
  authenticatedMcpToolDenyPolicy,
  authenticatedSourceIpPolicy,
} from "@k2/pomerium";

import { PAPERLESS_MCP_SERVICE_NAME, PAPERLESS_SERVICE_NAME } from "../../constants.js";

import { PaperlessDatabase } from "./database.js";
import { PaperlessDeployment } from "./deployment.js";
import { RedisDeployment } from "./redis.js";
import { PaperlessMcpTokenSecret, PaperlessProvisioningSecret, PaperlessSecret } from "./secret.js";
import { PaperlessMcpService, PaperlessService, RedisService } from "./service.js";
import { PaperlessSetup } from "./setup.js";

const PAPERLESS_HOST_PREFIX = "paperless";
const DOCUMENTS_PATH = "/mnt/data/documents/archive";
const DENIED_MCP_TOOLS = [
  "delete_tag",
  "bulk_edit_tags",
  "delete_correspondent",
  "bulk_edit_correspondents",
  "delete_document_type",
  "bulk_edit_document_types",
  "delete_custom_field",
  "bulk_edit_custom_fields",
];

export class Paperless extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const host = ApexDomain.of(this).subdomain(PAPERLESS_HOST_PREFIX);
    const appUrl = `https://${host}`;
    const secret = new PaperlessSecret(this, "secret");
    const provisioningSecret = new PaperlessProvisioningSecret(this, "provisioning-secret");
    const mcpTokenSecret = new PaperlessMcpTokenSecret(this, "mcp-token-secret");
    const database = new PaperlessDatabase(this, "database");

    new RedisDeployment(this, "redis", { secretName: secret.secretName });
    new RedisService(this, "redis-service");
    new PaperlessSetup(this, "setup", {
      appSecretName: secret.secretName,
      mcpTokenSecretName: mcpTokenSecret.secretName,
      provisioningSecretName: provisioningSecret.secretName,
    });
    new PaperlessDeployment(this, "deployment", {
      appUrl,
      credentialsSecretName: database.credentialsSecretName,
      mcpTokenSecretName: mcpTokenSecret.secretName,
      secretName: secret.secretName,
      volumes: {
        data: K2Volume.replicated({ name: "paperless-data", size: Size.gibibytes(4) }),
        documents: K2Volume.mountNfs({ path: DOCUMENTS_PATH }),
      },
    });
    new PaperlessService(this, "service");
    new PaperlessMcpService(this, "mcp-service");
    new AuthenticatedIngress(this, "ingress", {
      host,
      serviceName: PAPERLESS_SERVICE_NAME,
      servicePort: "http",
      passIdentityHeaders: true,
      policy: authenticatedSourceIpPolicy(),
    });
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host,
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: PAPERLESS_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedMcpToolDenyPolicy({ deniedTools: DENIED_MCP_TOOLS }),
    });
  }
}
