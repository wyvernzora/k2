import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { ZIGBEE2MQTT_SERVICE_NAME } from "../../constants.js";

import { Zigbee2MqttConfig } from "./config.js";
import { Zigbee2MqttDeployment } from "./deployment.js";
import { Zigbee2MqttService } from "./service.js";

const ZIGBEE2MQTT_HOST_PREFIX = "z2m";

export class Zigbee2Mqtt extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const host = ApexDomain.of(this).subdomain(ZIGBEE2MQTT_HOST_PREFIX);
    const config = new Zigbee2MqttConfig(this, "config", {
      url: `https://${host}`,
    });
    new Zigbee2MqttDeployment(this, "deployment", {
      configName: config.name,
      configChecksum: config.checksum,
      volumes: {
        data: K2Volume.replicated({ name: "zigbee2mqtt-data", size: Size.gibibytes(1) }),
      },
    });
    new Zigbee2MqttService(this, "service");
    new AuthenticatedIngress(this, "ingress", {
      name: "zigbee2mqtt",
      host,
      serviceName: ZIGBEE2MQTT_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
