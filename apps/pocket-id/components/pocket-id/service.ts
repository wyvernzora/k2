import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { POCKET_ID_HTTP_PORT, POCKET_ID_LABELS, POCKET_ID_SERVICE_NAME } from "../../lib/constants.js";

export class PocketIdService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: POCKET_ID_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "pocket-id-service-pods", { labels: POCKET_ID_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: POCKET_ID_HTTP_PORT }],
    });
  }
}
