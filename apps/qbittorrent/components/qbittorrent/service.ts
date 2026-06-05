import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  FLOOD_HTTP_PORT,
  FLOOD_SERVICE_NAME,
  QBITTORRENT_HTTP_PORT,
  QBITTORRENT_LABELS,
  QBITTORRENT_MCP_PORT,
  QBITTORRENT_MCP_SERVICE_NAME,
} from "../../constants.js";

const QBITTORRENT_SERVICE_NAME = "qbittorrent";

export class FloodService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: FLOOD_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "flood-service-pods", { labels: QBITTORRENT_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: FLOOD_HTTP_PORT }],
    });
  }
}

export class QbittorrentMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: QBITTORRENT_MCP_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "qbittorrent-mcp-service-pods", { labels: QBITTORRENT_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: QBITTORRENT_MCP_PORT }],
    });
  }
}

export class QbittorrentService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: QBITTORRENT_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "qbittorrent-service-pods", { labels: QBITTORRENT_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: QBITTORRENT_HTTP_PORT }],
    });
  }
}
