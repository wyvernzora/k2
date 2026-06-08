import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { UNIFI_NETWORK_MCP_LABELS, UNIFI_NETWORK_MCP_PORT, UNIFI_NETWORK_MCP_SERVICE_NAME } from "../../constants.js";

export class UnifiNetworkMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: UNIFI_NETWORK_MCP_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "unifi-network-mcp-service-pods", { labels: UNIFI_NETWORK_MCP_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: UNIFI_NETWORK_MCP_PORT }],
    });
  }
}
