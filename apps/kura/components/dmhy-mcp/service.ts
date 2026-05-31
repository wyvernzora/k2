import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { DMHY_MCP_LABELS, DMHY_MCP_PORT, DMHY_MCP_SERVICE_NAME } from "./labels.js";

export class DmhyMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: DMHY_MCP_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "dmhy-mcp-service-pods", { labels: DMHY_MCP_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: DMHY_MCP_PORT }],
    });
  }
}
