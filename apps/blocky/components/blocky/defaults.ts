import type { VlanConfig } from "@k2/cdk-lib";

import { BlockingGroup } from "./blocking-group.js";
import { ClientGroup } from "./client-group.js";

export const K8S_GATEWAY_ZONE = "wyvernzora.io";
export const K8S_GATEWAY_UPSTREAM = "tcp+udp:k8s-gateway.blocky.svc.cluster.local:53";

const INTERNAL_UPSTREAMS = [K8S_GATEWAY_UPSTREAM];

const INTERNAL_CLIENT_VLANS = new Set(["default", "privileged", "infrastructure"]);
const PUBLIC_CLIENT_VLANS = new Set(["sandbox"]);

export interface DefaultClientGroupsProps {
  readonly internalCidrs: string[];
  readonly vlans: VlanConfig[];
  readonly publicDnsServers: string[];
  readonly blocklistUrls: string[];
}

export function defaultClientGroups(props: DefaultClientGroupsProps): ClientGroup[] {
  const blockingGroup = new BlockingGroup({
    name: "default",
    blacklists: props.blocklistUrls,
  });

  return [
    ...props.internalCidrs.map(cidr => internalClientGroup(cidr, blockingGroup)),
    ...props.vlans.map(vlan => vlanClientGroup(vlan, blockingGroup, props.publicDnsServers)),
    publicClientGroup("default", blockingGroup, props.publicDnsServers),
  ];
}

function vlanClientGroup(vlan: VlanConfig, blockingGroup: BlockingGroup, publicDnsServers: string[]): ClientGroup {
  if (INTERNAL_CLIENT_VLANS.has(vlan.name)) {
    return internalClientGroup(vlan.cidr, blockingGroup);
  }
  if (PUBLIC_CLIENT_VLANS.has(vlan.name)) {
    return publicClientGroup(vlan.cidr, blockingGroup, publicDnsServers);
  }
  throw new Error(`Blocky resolver policy is not defined for VLAN ${vlan.name}`);
}

function internalClientGroup(name: string, blockingGroup: BlockingGroup): ClientGroup {
  return new ClientGroup({
    name,
    upstream: INTERNAL_UPSTREAMS,
    blockingGroups: [blockingGroup],
  });
}

function publicClientGroup(name: string, blockingGroup: BlockingGroup, publicDnsServers: string[]): ClientGroup {
  return new ClientGroup({
    name,
    upstream: publicDnsServers,
    blockingGroups: [blockingGroup],
  });
}
