import { k8s } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { POMERIUM_INGRESS_CLASS_NAME } from "./constants.js";

interface PomeriumIngressRouteProps {
  readonly host: string;
  readonly serviceName: string;
  readonly servicePort: number | string;
  readonly name?: string;
  readonly path?: string;
  readonly tlsSecretName?: string;
}

export interface AuthenticatedIngressProps extends PomeriumIngressRouteProps {
  readonly policy?: string;
}

export interface AuthenticatedMcpIngressProps extends AuthenticatedIngressProps {
  readonly mcpPath?: string;
}

// eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Ingress L2 cannot target a backend service by name without owning a Service construct.
export class AuthenticatedIngress extends k8s.KubeIngress {
  public constructor(scope: Construct, id: string, props: AuthenticatedIngressProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? id,
        annotations: authenticatedIngressAnnotations(props),
      },
      spec: ingressSpec(props),
    });
  }
}

// eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Ingress L2 cannot target a backend service by name without owning a Service construct.
export class AuthenticatedMcpIngress extends k8s.KubeIngress {
  public constructor(scope: Construct, id: string, props: AuthenticatedMcpIngressProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? id,
        annotations: authenticatedMcpIngressAnnotations(props),
      },
      spec: ingressSpec(props),
    });
  }
}

export type PublicIngressProps = PomeriumIngressRouteProps;

// eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Ingress L2 cannot target a backend service by name without owning a Service construct.
export class PublicIngress extends k8s.KubeIngress {
  public constructor(scope: Construct, id: string, props: PublicIngressProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? id,
        annotations: {
          "ingress.pomerium.io/allow_public_unauthenticated_access": "true",
        },
      },
      spec: ingressSpec(props),
    });
  }
}

function ingressSpec(props: PomeriumIngressRouteProps) {
  return {
    ingressClassName: POMERIUM_INGRESS_CLASS_NAME,
    tls: [ingressTls(props)],
    rules: [ingressRule(props)],
  };
}

function ingressTls(props: PomeriumIngressRouteProps) {
  if (props.tlsSecretName === undefined) {
    return { hosts: [props.host] };
  }
  return { hosts: [props.host], secretName: props.tlsSecretName };
}

function ingressRule(props: PomeriumIngressRouteProps) {
  return {
    host: props.host,
    http: {
      paths: [ingressPath(props)],
    },
  };
}

function ingressPath(props: PomeriumIngressRouteProps) {
  return {
    path: props.path ?? "/",
    pathType: "Prefix",
    backend: {
      service: backendService(props),
    },
  };
}

function backendService(props: PomeriumIngressRouteProps) {
  return {
    name: props.serviceName,
    port: servicePort(props.servicePort),
  };
}

function authenticatedIngressAnnotations(props: AuthenticatedIngressProps): Record<string, string> | undefined {
  if (props.policy === undefined) {
    return undefined;
  }
  return {
    "ingress.pomerium.io/policy": props.policy,
  };
}

function authenticatedMcpIngressAnnotations(props: AuthenticatedMcpIngressProps): Record<string, string> {
  return {
    ...authenticatedIngressAnnotations(props),
    "ingress.pomerium.io/mcp_server": "true",
    "ingress.pomerium.io/mcp_server_path": props.mcpPath ?? props.path ?? "/",
  };
}

function servicePort(port: number | string) {
  if (typeof port === "number") {
    return { number: port };
  }
  return { name: port };
}
