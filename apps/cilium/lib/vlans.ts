import type { IConstruct } from "constructs";

import { ClusterContext } from "@k2/cdk-lib";

export function vlanCidrs(scope: IConstruct, names: string[]): string[] {
  const vlans = ClusterContext.of(scope).config.network.vlans;
  const byName = new Map(vlans.map(vlan => [vlan.name, vlan.cidr]));

  return names.map(name => {
    const cidr = byName.get(name);
    if (cidr === undefined) {
      throw new Error(`Unknown VLAN "${name}". Known VLANs: ${vlans.map(vlan => vlan.name).join(", ")}`);
    }
    return cidr;
  });
}
