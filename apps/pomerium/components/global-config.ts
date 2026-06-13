import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";

import {
  Pomerium,
  PomeriumSpecCodecType,
  PomeriumSpecIdentityProviderProvider,
  type PomeriumProps,
  type PomeriumSpec,
} from "../crds/ingress.pomerium.io.js";
import {
  POMERIUM_AUTHENTICATE_HOST_PREFIX,
  POMERIUM_BOOTSTRAP_SECRET_NAME,
  POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME,
  POMERIUM_DATABASE_SECRET_NAME,
  POMERIUM_IDP_HOST_PREFIX,
  POMERIUM_IDP_SECRET_NAME,
  POMERIUM_NAMESPACE,
} from "../constants.js";

const CodecType = PomeriumSpecCodecType;
const IdentityProvider = PomeriumSpecIdentityProviderProvider;
export const MCP_ALLOWED_CLIENT_ID_DOMAINS = [
  "wyvernzora.io",
  "*.wyvernzora.io",
  "chatgpt.com",
  "chat.openai.com",
  "claude.ai",
];
export const MCP_CLIENT_METADATA_EGRESS_HOSTS = ["chatgpt.com", "chat.openai.com", "claude.ai"];

export class PomeriumGlobalConfig extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new PomeriumGlobal(this, "global", ApexDomain.of(this));
  }
}

class PomeriumGlobal extends Pomerium {
  public constructor(scope: Construct, id: string, apex: ApexDomain) {
    super(scope, id, globalProps(apex));
  }
}

function globalProps(apex: ApexDomain): PomeriumProps {
  return {
    metadata: { name: "global" },
    spec: globalSpec(apex),
  };
}

function globalSpec(apex: ApexDomain): PomeriumSpec {
  return {
    authenticate: {
      url: `https://${apex.subdomain(POMERIUM_AUTHENTICATE_HOST_PREFIX)}`,
    },
    certificates: [`${POMERIUM_NAMESPACE}/${POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME}`],
    codecType: CodecType.HTTP3,
    cookie: {
      expire: "168h",
    },
    identityProvider: {
      provider: IdentityProvider.OIDC,
      scopes: ["openid", "email", "profile"],
      secret: `${POMERIUM_NAMESPACE}/${POMERIUM_IDP_SECRET_NAME}`,
      url: `https://${apex.subdomain(POMERIUM_IDP_HOST_PREFIX)}`,
    },
    jwtClaimHeaders: {
      "X-Pomerium-Claim-Email": "email",
      "X-Pomerium-Claim-Preferred-Username": "preferred_username",
    },
    mcpAllowedClientIdDomains: MCP_ALLOWED_CLIENT_ID_DOMAINS,
    runtimeFlags: { mcp: true },
    secrets: `${POMERIUM_NAMESPACE}/${POMERIUM_BOOTSTRAP_SECRET_NAME}`,
    storage: {
      postgres: {
        secret: `${POMERIUM_NAMESPACE}/${POMERIUM_DATABASE_SECRET_NAME}`,
      },
    },
  };
}
