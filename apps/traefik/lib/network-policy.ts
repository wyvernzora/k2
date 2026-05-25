import type { Construct } from "constructs";

import { PrivateConnection, type PolicyEndpoint, type PortSpec } from "@k2/cilium";

const TRAEFIK_NAMESPACE = "traefik";

export const endpoints = {
  proxy(): PolicyEndpoint {
    return {
      name: "traefik",
      namespace: TRAEFIK_NAMESPACE,
      labels: {
        "app.kubernetes.io/instance": "traefik-traefik",
        "app.kubernetes.io/name": "traefik",
      },
    };
  },
};

export interface AllowTraefikToBackendProps {
  readonly backend: PolicyEndpoint;
  readonly ports: PortSpec[];
  readonly name?: string;
}

export class AllowTraefikToBackend extends PrivateConnection {
  public constructor(scope: Construct, id: string, props: AllowTraefikToBackendProps) {
    super(scope, id, {
      name: props.name,
      from: endpoints.proxy(),
      to: props.backend,
      ports: props.ports,
    });
  }
}
