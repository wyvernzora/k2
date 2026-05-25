import type { Construct } from "constructs";

import { ClusterContext, K2Chart, type LoadBalancerPoolConfig } from "@k2/cdk-lib";

import { BlockyConfig } from "./config.js";
import { BlockyDaemonSet } from "./daemon-set.js";
import { defaultClientGroups } from "./defaults.js";
import { BlockyService } from "./service.js";

export interface BlockyProps {
  readonly publicDnsServers: string[];
  readonly blocklistUrls: string[];
}

export class Blocky extends K2Chart {
  public constructor(scope: Construct, id: string, props: BlockyProps) {
    super(scope, id);

    const cluster = ClusterContext.of(this).config;
    const config = new BlockyConfig(this, "config", {
      clientGroups: defaultClientGroups({
        vlans: cluster.network.vlans,
        publicDnsServers: props.publicDnsServers,
        blocklistUrls: props.blocklistUrls,
      }),
    });
    const daemonSet = new BlockyDaemonSet(this, "daemon-set", {
      configName: config.name,
      configChecksum: config.checksum,
    });
    new BlockyService(this, "service", {
      loadBalancerIp: blockyLoadBalancerIp(cluster.loadBalancerPools),
      selector: daemonSet.selectorLabels,
    });
  }
}

function blockyLoadBalancerIp(pools: LoadBalancerPoolConfig[]): string {
  const pool = pools.find(candidate => candidate.name === "blocky");
  if (pool === undefined) {
    throw new Error("Blocky requires a loadBalancerPools entry named blocky");
  }

  const [address, mask] = pool.cidr.split("/");
  if (mask !== "32") {
    throw new Error("Blocky loadBalancerPools.blocky must be a single-IP /32 CIDR");
  }
  return address;
}
