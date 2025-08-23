import { Service, ServiceType } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { BlockyConfig, BlockyConfigProps } from "./config.js";
import { BlockyDeployment } from "./deployment.js";

export * from "./blocking-group.js";
export * from "./client-group.js";
export * from "./custom-dns.js";

export interface BlockyProps extends BlockyConfigProps {
  readonly replicas?: number;
  readonly serviceIp?: string;
}

export class Blocky extends Construct {
  public readonly service: Service;

  constructor(scope: Construct, id: string, props: BlockyProps) {
    super(scope, id);
    const config = new BlockyConfig(this, "config", props);
    const deployment = new BlockyDeployment(this, "depl", {
      config,
      replicas: props.replicas || 3,
    });
    this.service = deployment.exposeViaService({
      name: "blocky",
      serviceType: ServiceType.LOAD_BALANCER,
    });
    if (props.serviceIp) {
      this.service.metadata.addAnnotation("metallb.universe.tf/loadBalancerIPs", props.serviceIp);
    }
  }
}
