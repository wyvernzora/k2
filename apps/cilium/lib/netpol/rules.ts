import type {
  CiliumClusterwideNetworkPolicySpecsEgress,
  CiliumClusterwideNetworkPolicySpecsIngress,
} from "../../crds/cilium.io.js";

import { clusterwideEgressEntity, clusterwideIngressEntity } from "./entities.js";
import { clusterwideEgressPorts, clusterwideIngressPorts } from "./ports.js";
import { endpointSelector } from "./selectors.js";
import type { EgressPeer, EgressRule, IngressPeer, IngressRule } from "./types.js";

export function clusterwideIngressRule(rule: IngressRule): CiliumClusterwideNetworkPolicySpecsIngress {
  return {
    ...clusterwideIngressFrom(rule.from),
    toPorts: rule.ports === undefined ? undefined : clusterwideIngressPorts(rule.ports),
  };
}

export function clusterwideEgressRule(rule: EgressRule): CiliumClusterwideNetworkPolicySpecsEgress {
  return {
    ...clusterwideEgressTo(rule.to),
    toPorts: rule.ports === undefined ? undefined : clusterwideEgressPorts(rule.ports, rule.dns),
  };
}

function clusterwideIngressFrom(
  from: IngressPeer,
): Pick<CiliumClusterwideNetworkPolicySpecsIngress, "fromCidr" | "fromEndpoints" | "fromEntities"> {
  if ("endpoint" in from) {
    return { fromEndpoints: [endpointSelector(from.endpoint)] };
  }
  if ("cidr" in from) {
    return { fromCidr: [from.cidr] };
  }
  return { fromEntities: [clusterwideIngressEntity(from.entity)] };
}

function clusterwideEgressTo(
  to: EgressPeer,
): Pick<CiliumClusterwideNetworkPolicySpecsEgress, "toCidr" | "toEndpoints" | "toEntities" | "toFqdNs"> {
  if ("endpoint" in to) {
    return { toEndpoints: [endpointSelector(to.endpoint)] };
  }
  if ("cidr" in to) {
    return { toCidr: [to.cidr] };
  }
  if ("fqdn" in to) {
    return { toFqdNs: to.fqdn };
  }
  return { toEntities: [clusterwideEgressEntity(to.entity)] };
}
