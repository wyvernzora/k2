import { Construct } from "constructs";
import { stringify } from "yaml";
import dedent from "dedent-js";

import { ConfigMap } from "@k2/cdk-lib";

import { Mosquitto } from "../mosquitto/index.js";

export interface Zigbee2MqttConfigProps {
  readonly url: string;
  readonly coordinator: string;
  readonly mosquitto: Mosquitto;
}
type Props = Zigbee2MqttConfigProps;

export class Zigbee2MqttConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {});
    this.renderZigbee2MqttConfig(props);
    this.addData("init.sh", this.renderInitScript());
    this.addData("configuration.yaml", this.renderZigbee2MqttConfig(props));
  }

  private renderInitScript(): string {
    return dedent`
      #!/bin/sh

      # Initialize configuration if not present
      rm /app//data/configuration.yaml
      if [ ! -s "/app/data/configuration.yaml" ]; then
        echo "Configuration file not found, copying from templates"
        cp /init/configuration.yaml /app/data/configuration.yaml
      fi

      echo "== Configuration file contents =="
      cat /app/data/configuration.yaml
      echo "== =="
    `;
  }

  private renderZigbee2MqttConfig(props: Props) {
    return stringify({
      frontend: {
        enabled: true,
        url: props.url,
      },
      mqtt: {
        base_topic: "zigbee2mqtt",
        server: `mqtt://${props.mosquitto.hostname}`,
      },
      serial: {
        port: props.coordinator,
        adapter: "zstack",
      },
    });
  }
}
