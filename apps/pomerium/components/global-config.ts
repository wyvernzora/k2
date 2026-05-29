import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";

import { Pomerium, PomeriumSpecCodecType, PomeriumSpecIdentityProviderProvider } from "../crds/ingress.pomerium.io.js";
import {
  POMERIUM_AUTHENTICATE_HOST_PREFIX,
  POMERIUM_BOOTSTRAP_SECRET_NAME,
  POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME,
  POMERIUM_IDP_HOST_PREFIX,
  POMERIUM_IDP_SECRET_NAME,
  POMERIUM_NAMESPACE,
} from "../lib/constants.js";

const CodecType = PomeriumSpecCodecType;
const IdentityProvider = PomeriumSpecIdentityProviderProvider;

export class PomeriumGlobalConfig extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const apex = ApexDomain.of(this);

    new Pomerium(this, "global", {
      metadata: { name: "global" },
      spec: {
        authenticate: {
          url: `https://${apex.subdomain(POMERIUM_AUTHENTICATE_HOST_PREFIX)}`,
        },
        certificates: [`${POMERIUM_NAMESPACE}/${POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME}`],
        codecType: CodecType.HTTP3,
        identityProvider: {
          provider: IdentityProvider.OIDC,
          scopes: ["openid", "email", "profile"],
          secret: `${POMERIUM_NAMESPACE}/${POMERIUM_IDP_SECRET_NAME}`,
          url: `https://${apex.subdomain(POMERIUM_IDP_HOST_PREFIX)}`,
        },
        secrets: `${POMERIUM_NAMESPACE}/${POMERIUM_BOOTSTRAP_SECRET_NAME}`,
      },
    });
  }
}
