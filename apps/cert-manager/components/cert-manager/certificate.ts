import type { ApiObjectMetadata } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain } from "@k2/cdk-lib";

import { Certificate } from "../../crds/cert-manager.io.js";

import { DEFAULT_CERTIFICATE_SECRET_NAME, LETS_ENCRYPT_DNS_ZONE, LETS_ENCRYPT_PROD_ISSUER_NAME } from "./constants.js";

export interface DefaultWildcardCertificateProps {
  readonly metadata?: ApiObjectMetadata;
  readonly issuerName?: string;
  readonly secretName?: string;
  readonly domains?: string[];
}

/**
 * Default apex and wildcard certificate issued once in the cert-manager namespace.
 */
export class DefaultWildcardCertificate extends Certificate {
  public constructor(scope: Construct, id: string, props: DefaultWildcardCertificateProps = {}) {
    const { apexDomain } = ApexDomain.of(scope);
    const domains = props.domains ?? [`*.${apexDomain}`, LETS_ENCRYPT_DNS_ZONE, `*.${LETS_ENCRYPT_DNS_ZONE}`];
    const name = props.metadata?.name ?? DEFAULT_CERTIFICATE_SECRET_NAME;

    super(scope, id, {
      metadata: {
        ...props.metadata,
        name,
      },
      spec: {
        dnsNames: domains,
        issuerRef: {
          kind: "ClusterIssuer",
          name: props.issuerName ?? LETS_ENCRYPT_PROD_ISSUER_NAME,
        },
        secretName: props.secretName ?? DEFAULT_CERTIFICATE_SECRET_NAME,
      },
    });
  }
}
