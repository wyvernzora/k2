import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { MOSQUITTO_LABELS, MOSQUITTO_MQTT_PORT, MOSQUITTO_SERVICE_NAME } from "./labels.js";

export class MosquittoService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: MOSQUITTO_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "mosquitto-service-pods", { labels: MOSQUITTO_LABELS }),
      ports: [{ name: "mqtt", port: MOSQUITTO_MQTT_PORT, targetPort: MOSQUITTO_MQTT_PORT }],
    });
  }
}
