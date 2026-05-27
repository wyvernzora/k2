import type { Construct } from "constructs";

import { Namespace } from "@k2/cdk-lib";

import {
  CiliumNetworkPolicy,
  CiliumNetworkPolicySpecEgressToPortsPortsProtocol,
  CiliumNetworkPolicySpecEgressToEntities,
  CiliumNetworkPolicySpecIngressFromEntities,
  type CiliumNetworkPolicySpecEgress,
  type CiliumNetworkPolicySpecEgressToPorts,
  type CiliumNetworkPolicySpecEgressToPortsRulesDns,
} from "../../crds/cilium.io.js";

import { endpointSelector, namespaceSelector } from "./selectors.js";
import type { NamespaceBoundaryPolicyProps } from "./types.js";

const IngressEntity = CiliumNetworkPolicySpecIngressFromEntities;
const EgressEntity = CiliumNetworkPolicySpecEgressToEntities;
const PortProtocol = CiliumNetworkPolicySpecEgressToPortsPortsProtocol;

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
        egress: [
          { toEndpoints: [namespaceSelector(namespace)] },
          { toEntities: [EgressEntity.KUBE_HYPHEN_APISERVER] },
          ...(props.allowDns === false ? [] : [kubeDnsEgress()]),
        ],
      },
    });
  }
}

function kubeDnsEgress(): CiliumNetworkPolicySpecEgress {
  return {
    toEndpoints: [kubeDnsEndpoint()],
    toPorts: [kubeDnsPorts()],
  };
}

function kubeDnsEndpoint() {
  return endpointSelector({
    namespace: "kube-system",
    labels: { "k8s-app": "kube-dns" },
  });
}

function kubeDnsPorts(): CiliumNetworkPolicySpecEgressToPorts {
  return {
    ports: [
      { port: "53", protocol: PortProtocol.UDP },
      { port: "53", protocol: PortProtocol.TCP },
    ],
    rules: { dns: [allDnsNames()] },
  };
}

function allDnsNames(): CiliumNetworkPolicySpecEgressToPortsRulesDns {
  return { matchPattern: "*" };
}
