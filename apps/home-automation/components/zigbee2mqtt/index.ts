import { Construct } from "constructs";
import { Service, ServiceType } from "cdk8s-plus-28";

import { K2Volume, K2Volumes } from "@k2/cdk-lib";

import { Mosquitto } from "../mosquitto";

import { Zigbee2MqttConfig } from "./config";
import { Zigbee2MqttDeployment } from "./deployment";

export interface Zigbee2MqttProps {
  readonly url: string;
  readonly mosquitto: Mosquitto;
  readonly coordinator: string;
  readonly volumes?: Partial<K2Volumes<"data">>;
}

export class Zigbee2Mqtt extends Construct {
  readonly service: Service;

  constructor(scope: Construct, id: string, props: Zigbee2MqttProps) {
    super(scope, id);
    const config = new Zigbee2MqttConfig(this, "conf", { ...props });
    const deployment = new Zigbee2MqttDeployment(this, "depl", {
      config,
      volumes: {
        data: K2Volume.ephemeral(),
        ...props.volumes,
      },
    });
    props.mosquitto.networkPolicy.addIngressRule(deployment);
    this.service = deployment.exposeViaService({
      serviceType: ServiceType.CLUSTER_IP,
      ports: [
        {
          port: 80,
          targetPort: 8080,
        },
      ],
    });
  }
}
