import type { Construct } from "constructs";

import { PrivateConnection, type PolicyEndpoint, type PortSpec } from "@k2/cilium";

import { POMERIUM_LABELS, POMERIUM_NAMESPACE } from "./constants.js";

export const endpoints = {
  proxy(): PolicyEndpoint {
    return {
      name: "pomerium",
      namespace: POMERIUM_NAMESPACE,
      labels: POMERIUM_LABELS,
    };
  },
};

export interface AllowPomeriumToBackendProps {
  readonly backend: PolicyEndpoint;
  readonly ports: PortSpec[];
  readonly name?: string;
}

export class AllowPomeriumToBackend extends PrivateConnection {
  public constructor(scope: Construct, id: string, props: AllowPomeriumToBackendProps) {
    super(scope, id, {
      name: props.name,
      from: endpoints.proxy(),
      to: props.backend,
      ports: props.ports,
    });
  }
}
