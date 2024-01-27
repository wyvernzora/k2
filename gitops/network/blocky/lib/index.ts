import { Chart } from "cdk8s";
import { Service, ServiceType } from "cdk8s-plus-27";
import { Construct } from "constructs";
import { BlockyConfig } from "./config";
import { BlockyDeployment } from "./deployment";

export interface BlockyChartProps {
  readonly blockLists: string[];
}

export class BlockyChart extends Chart {
  public readonly service: Service;

  constructor(scope: Construct, id: string, props: BlockyChartProps) {
    super(scope, id);
    const config = new BlockyConfig(this, "config", props);
    const deployment = new BlockyDeployment(this, "depl", {
      config,
      replicas: 2,
    });
    this.service = deployment.exposeViaService({
      name: "blocky",
      serviceType: ServiceType.LOAD_BALANCER,
    });
    this.service.metadata.addAnnotation(
      "metallb.universe.tf/loadBalancerIPs",
      "10.10.10.8",
    );
  }
}
