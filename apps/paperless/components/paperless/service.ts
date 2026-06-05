import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  PAPERLESS_HTTP_PORT,
  PAPERLESS_LABELS,
  PAPERLESS_MCP_PORT,
  PAPERLESS_MCP_SERVICE_NAME,
  PAPERLESS_SERVICE_NAME,
  REDIS_LABELS,
  REDIS_PORT,
} from "../../constants.js";

const REDIS_SERVICE_NAME = "paperless-redis";

export class PaperlessService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: PAPERLESS_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "paperless-service-pods", { labels: PAPERLESS_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: PAPERLESS_HTTP_PORT }],
    });
  }
}

export class PaperlessMcpService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: PAPERLESS_MCP_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "paperless-mcp-service-pods", { labels: PAPERLESS_LABELS }),
      ports: [{ name: "mcp", port: 80, targetPort: PAPERLESS_MCP_PORT }],
    });
  }
}

export class RedisService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: REDIS_SERVICE_NAME },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "redis-service-pods", { labels: REDIS_LABELS }),
      ports: [{ name: "redis", port: REDIS_PORT, targetPort: REDIS_PORT }],
    });
  }
}
