import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";
import { stringify } from "yaml";

import { MOSQUITTO_MQTT_PORT, MOSQUITTO_SERVICE_NAME } from "../../constants.js";

const CONFIG_MAP_NAME = "zigbee2mqtt-init";
const ZIGBEE_COORDINATOR = "tcp://10.10.229.62:6638";

export interface Zigbee2MqttConfigProps {
  readonly url: string;
}

export class Zigbee2MqttConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string, props: Zigbee2MqttConfigProps) {
    const data = configData(props.url);
    super(scope, id, {
      metadata: { name: CONFIG_MAP_NAME },
      data,
    });
    this.checksum = createHash("sha256").update(JSON.stringify(data)).digest("hex");
  }
}

function configData(url: string): Record<string, string> {
  return {
    "init.sh": initScript(),
    "configuration.yaml": configurationYaml(url),
  };
}

function initScript(): string {
  return dedent`
    #!/bin/sh
    set -eu

    if [ ! -s /app/data/configuration.yaml ]; then
      cp /init/configuration.yaml /app/data/configuration.yaml
    fi
  `;
}

function configurationYaml(url: string): string {
  return stringify({
    frontend: {
      enabled: true,
      url,
    },
    mqtt: {
      base_topic: "zigbee2mqtt",
      server: `mqtt://${MOSQUITTO_SERVICE_NAME}:${MOSQUITTO_MQTT_PORT}`,
    },
    serial: {
      port: ZIGBEE_COORDINATOR,
      adapter: "zstack",
    },
  });
}
