import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  KURA_HTTP_PORT,
  KURA_LABELS,
  KURA_MCP_PORT,
  KURA_MCP_SERVICE_NAME,
  KURA_SERVICE_NAME,
} from "../../constants.js";

export class KuraService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: KURA_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "kura-service-pods", { labels: KURA_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: KURA_HTTP_PORT }],
    });
  }
}

export class KuraMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: KURA_MCP_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "kura-mcp-service-pods", { labels: KURA_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: KURA_MCP_PORT }],
    });
  }
}
