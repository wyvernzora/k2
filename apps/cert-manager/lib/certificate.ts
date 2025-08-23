import { ApexDomain } from "cdk-lib/context";
import { Certificate, ClusterIssuer, Issuer } from "../crds/cert-manager.io";
import { Construct } from "constructs";

const NAMESPACES = "k2-auth,k2-network,plex";

export interface ReplicatedCertificateProps {
  readonly issuer: Issuer | ClusterIssuer;
}

/**
 * Since K2 uses a wildcard certificate, there is no point in issuing it over and
 * over again in different namespaces. Therefore this app include replicator, which
 * copies the generated certificate secret across namespaces. This construct sets up
 * the certificate for replication.
 */
export class K2Certificate extends Certificate {
  /**
   * Name of the K2 cluster's default certificate secret.
   * This is replicated across allow-listed namespaces.
   */
  public static readonly Name = "default-certificate";

  constructor(scope: Construct, id: string, props: ReplicatedCertificateProps) {
    const { apexDomain: domain } = ApexDomain.of(scope);
    super(scope, id, {
      metadata: {
        name: K2Certificate.Name,
      },
      spec: {
        commonName: `*.${domain}`,
        dnsNames: [`*.${domain}`],
        issuerRef: {
          kind: props.issuer.kind,
          name: props.issuer.name,
        },
        secretName: K2Certificate.Name,
        secretTemplate: {
          annotations: {
            ...reflectorAnnotation("allowed", "true"),
            ...reflectorAnnotation("allowed-namespaces", NAMESPACES),
            ...reflectorAnnotation("auto-enabled", "true"),
            ...reflectorAnnotation("auto-namespace", NAMESPACES),
          },
        },
      },
    });
  }
}

function reflectorAnnotation(key: string, value: string) {
  return {
    [`reflector.v1.k8s.emberstack.com/reflection-${key}`]: value,
  };
}
