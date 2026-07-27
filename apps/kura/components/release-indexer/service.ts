import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { KURA_RELEASE_INDEXER_HTTP_PORT, KURA_RELEASE_INDEXER_LABELS } from "../../constants.js";

export class ReleaseIndexerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "kura-release-indexer" },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "release-indexer-service-pods", { labels: KURA_RELEASE_INDEXER_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: KURA_RELEASE_INDEXER_HTTP_PORT }],
    });
  }
}

export class ReleaseIndexerMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "kura-release-indexer-mcp" },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "release-indexer-mcp-service-pods", { labels: KURA_RELEASE_INDEXER_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: KURA_RELEASE_INDEXER_HTTP_PORT }],
    });
  }
}
