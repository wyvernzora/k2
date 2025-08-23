import { Construct } from "constructs";
import { NetworkPolicy, Service, ServiceType } from "cdk8s-plus-28";

import { K2Volume } from "@k2/cdk-lib";

import { MosquittoDeployment, MosquittoDeploymentProps } from "./deployment";
import { MosquittoConfig } from "./config";

export interface MosquittoProps {
  readonly volumes?: Partial<MosquittoDeploymentProps["volumes"]>;
}

export class Mosquitto extends Construct {
  readonly service: Service;
  readonly networkPolicy: NetworkPolicy;

  constructor(scope: Construct, id: string, props: MosquittoProps) {
    super(scope, id);
    const config = new MosquittoConfig(this, "conf", {});
    const deployment = new MosquittoDeployment(this, "depl", {
      config,
      volumes: {
        data: K2Volume.ephemeral(),
        logs: K2Volume.ephemeral(),
        ...props.volumes,
      },
    });
    this.networkPolicy = deployment.networkPolicy;
    this.service = deployment.exposeViaService({
      serviceType: ServiceType.CLUSTER_IP,
    });
  }

  public get hostname(): string {
    return `${this.service.name}.${this.service.metadata.namespace}.svc.cluster.local`;
  }
}
