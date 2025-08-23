import { Chart } from "cdk8s";
import { Deployment, Service, ServiceType } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { PlexDeployment, PlexDeploymentProps } from "./deployment";

export interface PlexChartProps {
  readonly host: string;
  readonly volumes: PlexDeploymentProps["volumes"];
}

export class PlexChart extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;

  constructor(scope: Construct, id: string, props: PlexChartProps) {
    super(scope, id, {});

    this.deployment = new PlexDeployment(this, "depl", { ...props });
    this.service = this.deployment.exposeViaService({
      serviceType: ServiceType.LOAD_BALANCER,
      ports: [
        {
          name: "http",
          port: 80,
        },
        {
          name: "https",
          port: 443,
        },
        {
          name: "dlna",
          port: 1900,
        },
        {
          name: "mdns",
          port: 5353,
        },
        {
          name: "gdm-32410",
          port: 32410,
        },
        {
          name: "gdm-32412",
          port: 32412,
        },
        {
          name: "gdm-32413",
          port: 32413,
        },
        {
          name: "gdm-32414",
          port: 32414,
        },
      ],
    });
    this.service.metadata.addAnnotation("coredns.io/hostname", props.host);
  }
}
