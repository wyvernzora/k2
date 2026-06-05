import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { HOMER_HTTP_PORT, HOMER_LABELS, HOMER_SERVICE_NAME } from "../../constants.js";

export class HomerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: HOMER_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "homer-service-pods", { labels: HOMER_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: HOMER_HTTP_PORT }],
    });
  }
}
