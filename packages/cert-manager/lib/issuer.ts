import { K2Secret } from "@k2/1password";
import { ClusterIssuer } from "@k2/cert-manager/crds";
import { Construct } from "constructs";

const EMAIL = "wyvernzora+letsencrypt@gmail.com";
const DOMAIN = "wyvernzora.io";
const AWS_REGION = "us-west-2";
const LE_ACME_PROD = "https://acme-v02.api.letsencrypt.org/directory";

export interface K2IssuerProps {
  readonly credentials: K2Secret;
}

/**
 * K2 cluster issues that uses Let's Encrypt, DNS01 and Route53 for ACME
 * challenge-response.
 */
export class K2Issuer extends ClusterIssuer {
  /**
   * Name of the default instance of the issuer in K2 cluster.
   */
  public static readonly Name: string = "letsencrypt-prod";

  constructor(scope: Construct, id: string, props: K2IssuerProps) {
    super(scope, id, {
      metadata: {
        name: K2Issuer.Name,
      },
      spec: {
        acme: {
          email: EMAIL,
          server: LE_ACME_PROD,
          privateKeySecretRef: {
            name: `${K2Issuer.Name}-privkey`,
          },
          solvers: [
            {
              selector: {
                dnsZones: [DOMAIN],
              },
              dns01: {
                route53: {
                  region: AWS_REGION,
                  accessKeyIdSecretRef: {
                    name: props.credentials.name,
                    key: "access-key-id",
                  },
                  secretAccessKeySecretRef: {
                    name: props.credentials.name,
                    key: "secret-access-key",
                  },
                },
              },
            },
          ],
        },
      },
    });
  }
}
