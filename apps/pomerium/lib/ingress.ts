import { KubeIngress } from "cdk8s-plus-32/lib/imports/k8s.js";
import type { Construct } from "constructs";

import { POMERIUM_INGRESS_CLASS_NAME } from "./constants.js";

export interface AuthenticatedIngressProps {
  readonly host: string;
  readonly serviceName: string;
  readonly servicePort: number | string;
  readonly name?: string;
  readonly path?: string;
  readonly policy?: string;
  readonly tlsSecretName?: string;
}

export class AuthenticatedIngress extends KubeIngress {
  public constructor(scope: Construct, id: string, props: AuthenticatedIngressProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? id,
        annotations: ingressAnnotations(props),
      },
      spec: ingressSpec(props),
    });
  }
}

function ingressSpec(props: AuthenticatedIngressProps) {
  return {
    ingressClassName: POMERIUM_INGRESS_CLASS_NAME,
    tls: [{ hosts: [props.host], secretName: props.tlsSecretName }],
    rules: [ingressRule(props)],
  };
}

function ingressRule(props: AuthenticatedIngressProps) {
  return {
    host: props.host,
    http: {
      paths: [ingressPath(props)],
    },
  };
}

function ingressPath(props: AuthenticatedIngressProps) {
  return {
    path: props.path ?? "/",
    pathType: "Prefix",
    backend: {
      service: backendService(props),
    },
  };
}

function backendService(props: AuthenticatedIngressProps) {
  return {
    name: props.serviceName,
    port: servicePort(props.servicePort),
  };
}

function ingressAnnotations(props: AuthenticatedIngressProps): Record<string, string> | undefined {
  if (props.policy === undefined) {
    return undefined;
  }
  return {
    "ingress.pomerium.io/policy": props.policy,
  };
}

function servicePort(port: number | string) {
  if (typeof port === "number") {
    return { number: port };
  }
  return { name: port };
}
