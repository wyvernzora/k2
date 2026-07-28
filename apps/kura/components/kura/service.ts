import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  KURA_GATEWAY_HTTP_PORT,
  KURA_GATEWAY_LABELS,
  KURA_LIBRARY_MANAGER_HTTP_PORT,
  KURA_LIBRARY_MANAGER_LABELS,
  KURA_LIBRARY_MANAGER_SERVICE_NAME,
  KURA_SERVICE_NAME,
} from "../../constants.js";

export class KuraService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: KURA_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "kura-service-pods", { labels: KURA_GATEWAY_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: KURA_GATEWAY_HTTP_PORT }],
    });
  }
}

export class LibraryManagerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: KURA_LIBRARY_MANAGER_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "library-manager-service-pods", { labels: KURA_LIBRARY_MANAGER_LABELS }),
      ports: [
        {
          name: "http",
          port: KURA_LIBRARY_MANAGER_HTTP_PORT,
          targetPort: KURA_LIBRARY_MANAGER_HTTP_PORT,
        },
      ],
    });
  }
}
