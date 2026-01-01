import { Construct } from "constructs";
import { Deployment, DeploymentStrategy, Volume, VolumeMount } from "cdk8s-plus-32";

import { K2Volumes, oci } from "@k2/cdk-lib";

import { Zigbee2MqttConfig } from "./config.js";

export interface Zigbee2MqttDeploymentProps {
  readonly config: Zigbee2MqttConfig;
  readonly volumes: K2Volumes<"data">;
}
type Props = Zigbee2MqttDeploymentProps;

export class Zigbee2MqttDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    const dataVolume = props.volumes.data(this, "vol-data").mount(this, {
      path: "/app/data",
    });
    this.addInitConfigContainer(props, dataVolume);
    this.addZigbee2MqttContainer(dataVolume);
    props.config.addChecksumTo(this);
  }

  private addZigbee2MqttContainer(dataVolume: VolumeMount) {
    this.addContainer({
      name: "zigbee2mqtt",
      image: oci`koenkk/zigbee2mqtt:2.7.2`,
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: [dataVolume],
      securityContext: {
        ensureNonRoot: false,
      },
    });
  }

  private addInitConfigContainer(props: Props, dataVolume: VolumeMount) {
    this.addInitContainer({
      name: "setup-config",
      image: oci`bash:latest`,
      command: ["bash", "-c"],
      args: ["bash /init/init.sh"],
      securityContext: {
        ensureNonRoot: false,
        user: 0,
      },
      volumeMounts: [
        dataVolume,
        {
          volume: Volume.fromConfigMap(this, "vol-init", props.config),
          path: "/init",
        },
      ],
    });
  }
}
