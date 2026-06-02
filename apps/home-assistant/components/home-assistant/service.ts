import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { HOME_ASSISTANT_HTTP_PORT, HOME_ASSISTANT_LABELS, HOME_ASSISTANT_SERVICE_NAME } from "./labels.js";

export class HomeAssistantService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: HOME_ASSISTANT_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "home-assistant-service-pods", { labels: HOME_ASSISTANT_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: HOME_ASSISTANT_HTTP_PORT }],
    });
  }
}
