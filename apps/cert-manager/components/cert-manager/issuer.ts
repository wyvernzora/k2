import type { ApiObjectMetadata } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, ClusterContext } from "@k2/cdk-lib";

import {
  ClusterIssuer,
  type ClusterIssuerSpecAcme as AcmeSpec,
  type ClusterIssuerSpecAcmeSolvers as AcmeSolver,
  type ClusterIssuerSpecAcmeSolversDns01Route53 as Route53Config,
  type ClusterIssuerSpecAcmeSolversDns01Route53Auth as Route53Auth,
} from "../../crds/cert-manager.io.js";

import {
  LETS_ENCRYPT_EMAIL,
  LETS_ENCRYPT_PROD_ACME_SERVER,
  LETS_ENCRYPT_PROD_ISSUER_NAME,
  ROUTE53_DNS01_ROLE_NAME,
  ROUTE53_DNS01_SERVICE_ACCOUNT_NAME,
  ROUTE53_HOSTED_ZONE_ID,
} from "./constants.js";

export interface LetsEncryptClusterIssuerProps {
  readonly metadata?: ApiObjectMetadata;
  readonly email?: string;
  readonly server?: string;
  readonly privateKeySecretName?: string;
  readonly dnsZones?: string[];
  readonly hostedZoneId?: string;
  readonly region?: string;
  readonly route53RoleName?: string;
  readonly route53ServiceAccountName?: string;
}

/**
 * Cluster-wide Let's Encrypt issuer for the K2 wildcard certificate.
 */
export class LetsEncryptClusterIssuer extends ClusterIssuer {
  public constructor(scope: Construct, id: string, props: LetsEncryptClusterIssuerProps = {}) {
    const { apexDomain } = ApexDomain.of(scope);
    const cluster = ClusterContext.of(scope).config;
    const region = props.region ?? cluster.aws?.region;
    const roleName = props.route53RoleName ?? ROUTE53_DNS01_ROLE_NAME;
    const roleArn = route53RoleArn(cluster.aws?.accountId, roleName);

    if (region === undefined || region.trim().length === 0) {
      throw new Error("LetsEncryptClusterIssuer: region is required; set props.region or clusters/v3.yaml aws.region");
    }

    const name = props.metadata?.name ?? LETS_ENCRYPT_PROD_ISSUER_NAME;

    super(scope, id, {
      metadata: {
        ...props.metadata,
        name,
      },
      spec: {
        acme: letsEncryptAcmeSpec({
          email: props.email ?? LETS_ENCRYPT_EMAIL,
          server: props.server ?? LETS_ENCRYPT_PROD_ACME_SERVER,
          privateKeySecretName: props.privateKeySecretName ?? `${name}-privkey`,
          solver: route53Dns01Solver({
            dnsZones: props.dnsZones ?? [apexDomain],
            hostedZoneId: props.hostedZoneId ?? ROUTE53_HOSTED_ZONE_ID,
            region,
            roleArn,
            serviceAccountName: props.route53ServiceAccountName ?? ROUTE53_DNS01_SERVICE_ACCOUNT_NAME,
          }),
        }),
      },
    });
  }
}

interface LetsEncryptAcmeSpecProps {
  readonly email: string;
  readonly server: string;
  readonly privateKeySecretName: string;
  readonly solver: AcmeSolver;
}

function letsEncryptAcmeSpec(props: LetsEncryptAcmeSpecProps): AcmeSpec {
  return {
    email: props.email,
    server: props.server,
    privateKeySecretRef: {
      name: props.privateKeySecretName,
    },
    solvers: [props.solver],
  };
}

interface Route53Dns01SolverProps {
  readonly dnsZones: string[];
  readonly hostedZoneId: string;
  readonly region: string;
  readonly roleArn: string;
  readonly serviceAccountName: string;
}

function route53Dns01Solver(props: Route53Dns01SolverProps): AcmeSolver {
  return {
    selector: {
      dnsZones: props.dnsZones,
    },
    dns01: {
      route53: route53Config(props),
    },
  };
}

function route53Config(props: Route53Dns01SolverProps): Route53Config {
  return {
    region: props.region,
    hostedZoneId: props.hostedZoneId,
    role: props.roleArn,
    auth: route53KubernetesAuth(props.serviceAccountName),
  };
}

function route53KubernetesAuth(serviceAccountName: string): Route53Auth {
  return {
    kubernetes: {
      serviceAccountRef: {
        name: serviceAccountName,
      },
    },
  };
}

function route53RoleArn(accountId: string | undefined, roleName: string): string {
  if (accountId === undefined) {
    throw new Error("LetsEncryptClusterIssuer: accountId is required; set clusters/v3.yaml aws.accountId");
  }
  if (roleName.trim().length === 0) {
    throw new Error("LetsEncryptClusterIssuer: route53RoleName must not be empty");
  }
  return `arn:aws:iam::${accountId}:role/${roleName}`;
}
