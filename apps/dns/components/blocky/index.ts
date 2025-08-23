import { Service, ServiceType } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { BlockyConfig, BlockyConfigProps } from "./config";
import { BlockyDeployment } from "./deployment";

export { BlockingGroup, BlockingGroupProps } from "./blocking-group";
export { ClientGroup, ClientGroupProps } from "./client-group";
export { CustomDns, CustomDnsProps } from "./custom-dns";

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
