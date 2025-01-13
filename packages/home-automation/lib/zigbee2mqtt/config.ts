import { Construct } from "constructs";
import { ConfigMap } from "@k2/cdk-lib";
import { stringify } from "yaml";
import { Mosquitto } from "../mosquitto";

export interface Zigbee2MqttConfigProps {
  readonly coordinator: string;
  readonly mosquitto: Mosquitto;
}
type Props = Zigbee2MqttConfigProps;

export class Zigbee2MqttConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {});
    this.renderZigbee2MqttConfig(props);
    this.addData("configuration.yaml", this.renderZigbee2MqttConfig(props));
  }

  private renderZigbee2MqttConfig(props: Props) {
    return stringify({
      frontend: {
        enabled: true,
      },
      mqtt: {
        base_topic: "zigbee2mqtt",
        server: `mqtt://${props.mosquitto.hostname}`,
      },
      serial: {
        port: props.coordinator,
      },
    });
  }
}
