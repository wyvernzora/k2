import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { N8N_HTTP_PORT, N8N_LABELS, N8N_SERVICE_NAME } from "./labels.js";

export class N8NService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: N8N_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "n8n-service-pods", { labels: N8N_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: N8N_HTTP_PORT }],
    });
  }
}
