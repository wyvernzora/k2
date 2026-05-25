import type { VlanConfig } from "@k2/cdk-lib";

import { BlockingGroup } from "./blocking-group.js";
import { ClientGroup } from "./client-group.js";

const INTERNAL_UPSTREAMS = ["tcp+udp:k8s-gateway.blocky.svc.cluster.local:53"];

const INTERNAL_CLIENT_VLANS = new Set(["default", "privileged", "infrastructure"]);
const PUBLIC_CLIENT_VLANS = new Set(["sandbox"]);

export interface DefaultClientGroupsProps {
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
    ...props.vlans.map(vlan => vlanClientGroup(vlan, blockingGroup, props.publicDnsServers)),
    publicClientGroup("default", blockingGroup, props.publicDnsServers),
  ];
}

function vlanClientGroup(vlan: VlanConfig, blockingGroup: BlockingGroup, publicDnsServers: string[]): ClientGroup {
  if (INTERNAL_CLIENT_VLANS.has(vlan.name)) {
    return new ClientGroup({
      name: vlan.cidr,
      upstream: INTERNAL_UPSTREAMS,
      blockingGroups: [blockingGroup],
    });
  }
  if (PUBLIC_CLIENT_VLANS.has(vlan.name)) {
    return publicClientGroup(vlan.cidr, blockingGroup, publicDnsServers);
  }
  throw new Error(`Blocky resolver policy is not defined for VLAN ${vlan.name}`);
}

function publicClientGroup(name: string, blockingGroup: BlockingGroup, publicDnsServers: string[]): ClientGroup {
  return new ClientGroup({
    name,
    upstream: publicDnsServers,
    blockingGroups: [blockingGroup],
  });
}
