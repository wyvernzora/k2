import type { Construct } from "constructs";

import { Namespace } from "@k2/cdk-lib";

import {
  CiliumNetworkPolicy,
  CiliumNetworkPolicySpecEgressToEntities,
  CiliumNetworkPolicySpecIngressFromEntities,
} from "../../crds/cilium.io.js";

import { namespaceSelector } from "./selectors.js";
import type { NamespaceBoundaryPolicyProps } from "./types.js";

const IngressEntity = CiliumNetworkPolicySpecIngressFromEntities;
const EgressEntity = CiliumNetworkPolicySpecEgressToEntities;

export class NamespaceBoundaryPolicy extends CiliumNetworkPolicy {
  public constructor(scope: Construct, id = "namespace-boundary", props: NamespaceBoundaryPolicyProps = {}) {
    const namespace = props.namespace ?? Namespace.of(scope).namespace;
    super(scope, id, {
      metadata: {
        name: props.name ?? "namespace-boundary",
      },
      spec: {
        endpointSelector: {},
        ingress: [
          { fromEndpoints: [namespaceSelector(namespace)] },
          { fromEntities: [IngressEntity.KUBE_HYPHEN_APISERVER] },
        ],
        egress: [{ toEndpoints: [namespaceSelector(namespace)] }, { toEntities: [EgressEntity.KUBE_HYPHEN_APISERVER] }],
      },
    });
  }
}
