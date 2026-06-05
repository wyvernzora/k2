import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { ZIGBEE2MQTT_HTTP_PORT, ZIGBEE2MQTT_LABELS, ZIGBEE2MQTT_SERVICE_NAME } from "../../constants.js";

export class Zigbee2MqttService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: ZIGBEE2MQTT_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "zigbee2mqtt-service-pods", { labels: ZIGBEE2MQTT_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: ZIGBEE2MQTT_HTTP_PORT }],
    });
  }
}
