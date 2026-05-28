import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";

import { Pomerium, PomeriumSpecCodecType } from "../crds/ingress.pomerium.io.js";
import {
  POMERIUM_BOOTSTRAP_SECRET_NAME,
  POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME,
  POMERIUM_NAMESPACE,
} from "../lib/constants.js";

const CodecType = PomeriumSpecCodecType;

export class PomeriumGlobalConfig extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new Pomerium(this, "global", {
      metadata: { name: "global" },
      spec: {
        authenticate: {
          url: `https://${ApexDomain.of(this).subdomain("authenticate")}`,
        },
        certificates: [`${POMERIUM_NAMESPACE}/${POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME}`],
        codecType: CodecType.HTTP3,
        secrets: `${POMERIUM_NAMESPACE}/${POMERIUM_BOOTSTRAP_SECRET_NAME}`,
      },
    });
  }
}
