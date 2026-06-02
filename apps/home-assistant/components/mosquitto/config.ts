import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { MOSQUITTO_MQTT_PORT } from "./labels.js";

const CONFIG_MAP_NAME = "mosquitto-config";

export class MosquittoConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string) {
    const config = mosquittoConfig();
    super(scope, id, {
      metadata: { name: CONFIG_MAP_NAME },
      data: {
        "mosquitto.conf": config,
      },
    });
    this.checksum = createHash("sha256").update(config).digest("hex");
  }
}

function mosquittoConfig(): string {
  return dedent`
    listener ${MOSQUITTO_MQTT_PORT}
    allow_anonymous true
    persistence true
    persistence_location /mosquitto/data/
    log_dest stdout
  `;
}
