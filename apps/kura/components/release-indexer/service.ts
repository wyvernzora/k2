import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  KURA_RELEASE_INDEXER_HTTP_PORT,
  KURA_RELEASE_INDEXER_LABELS,
  KURA_RELEASE_INDEXER_SERVICE_NAME,
} from "../../constants.js";

export class ReleaseIndexerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: KURA_RELEASE_INDEXER_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "release-indexer-service-pods", { labels: KURA_RELEASE_INDEXER_LABELS }),
      ports: [
        {
          name: "http",
          port: KURA_RELEASE_INDEXER_HTTP_PORT,
          targetPort: KURA_RELEASE_INDEXER_HTTP_PORT,
        },
      ],
    });
  }
}
