import type { Construct } from "constructs";

import { ClusterContext, K2Chart } from "@k2/cdk-lib";

import { BlockyConfig } from "./config.js";
import { BlockyDaemonSet } from "./daemon-set.js";
import { defaultClientGroups } from "./defaults.js";
import { blockyLoadBalancerIp } from "./load-balancer.js";
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
        internalCidrs: [cluster.kubernetes.subnets.pods, cluster.kubernetes.subnets.services],
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
