import { K2Volumes, oci } from "@k2/cdk-lib";
import { Construct } from "constructs";
import { Deployment, DeploymentStrategy, Volume } from "cdk8s-plus-28";
import { Zigbee2MqttConfig } from "./config";

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
    props.config.addChecksumTo(this);
    this.addZigbee2MqttContainer(props);
  }

  private addZigbee2MqttContainer(props: Props) {
    this.addContainer({
      name: "zigbee2mqtt",
      image: oci`koenkk/zigbee2mqtt:1.39.1`,
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: [
        props.volumes.data(this, "vol-data").mount(this, {
          path: "/app/data",
        }),
        {
          volume: Volume.fromConfigMap(this, "vol-conf", props.config),
          path: "/app/data/configuration.yaml",
          subPath: "configuration.yaml",
        },
      ],
      securityContext: {
        ensureNonRoot: false,
      },
    });
  }
}
