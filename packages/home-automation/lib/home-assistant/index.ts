import { Construct } from "constructs";
import { K2Volume, K2Volumes } from "@k2/cdk-lib";
import { Mosquitto } from "../mosquitto";
import { HomeAssistantDeployment } from "./deployment";
import { Service, ServiceType } from "cdk8s-plus-28";
import { HomeAssistantConfig } from "./config";

export interface HomeAssistantProps {
  readonly mosquitto: Mosquitto;
  readonly volumes?: Partial<K2Volumes<"config">>;
}

export class HomeAssistant extends Construct {
  readonly service: Service;

  constructor(scope: Construct, id: string, props: HomeAssistantProps) {
    super(scope, id);
    const config = new HomeAssistantConfig(this, "conf");
    const deployment = new HomeAssistantDeployment(this, "depl", {
      config,
      volumes: {
        config: K2Volume.ephemeral(),
        ...props.volumes,
      },
    });
    this.service = deployment.exposeViaService({
      serviceType: ServiceType.CLUSTER_IP,
      ports: [
        {
          port: 80,
          targetPort: 8123,
        },
      ],
    });
    props.mosquitto.networkPolicy.addIngressRule(deployment);
  }
}
