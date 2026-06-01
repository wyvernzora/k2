import {
  CiliumClusterwideNetworkPolicySpecsEgressToPortsPortsProtocol,
  CiliumClusterwideNetworkPolicySpecsIngressDenyToPortsPortsProtocol,
  CiliumClusterwideNetworkPolicySpecsIngressToPortsPortsProtocol,
  type CiliumClusterwideNetworkPolicySpecsEgressToPorts,
  type CiliumClusterwideNetworkPolicySpecsEgressToPortsRulesDns,
  type CiliumClusterwideNetworkPolicySpecsIngressDenyToPorts,
  type CiliumClusterwideNetworkPolicySpecsIngressToPorts,
} from "../../crds/cilium.io.js";

import type { FqdnMatch, PortSpec } from "./types.js";

type EgressToPorts = CiliumClusterwideNetworkPolicySpecsEgressToPorts;
type EgressDnsRule = CiliumClusterwideNetworkPolicySpecsEgressToPortsRulesDns;
type IngressToPorts = CiliumClusterwideNetworkPolicySpecsIngressToPorts;
type IngressDenyToPorts = CiliumClusterwideNetworkPolicySpecsIngressDenyToPorts;

export function clusterwideIngressPorts(portSpecs: PortSpec[]): IngressToPorts[] {
  return [
    {
      ports: portSpecs.map(port => ({
        protocol: CiliumClusterwideNetworkPolicySpecsIngressToPortsPortsProtocol[port.protocol],
        port: String(port.port),
      })),
    },
  ];
}

export function clusterwideIngressDenyPorts(portSpecs: PortSpec[]): IngressDenyToPorts[] {
  return [
    {
      ports: portSpecs.map(port => ({
        protocol: CiliumClusterwideNetworkPolicySpecsIngressDenyToPortsPortsProtocol[port.protocol],
        port: String(port.port),
      })),
    },
  ];
}

export function clusterwideEgressPorts(portSpecs: PortSpec[], dnsRules?: FqdnMatch[]): EgressToPorts[] {
  return [
    {
      ports: portSpecs.map(port => ({
        protocol: CiliumClusterwideNetworkPolicySpecsEgressToPortsPortsProtocol[port.protocol],
        port: String(port.port),
      })),
      rules: dnsRules === undefined ? undefined : { dns: dnsRules.map(dnsRule) },
    },
  ];
}

function dnsRule(match: FqdnMatch): EgressDnsRule {
  return {
    matchName: match.matchName,
    matchPattern: match.matchPattern,
  };
}
