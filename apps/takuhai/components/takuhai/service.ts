import { Pods, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  TAKUHAI_CRAWLER_DMHY_LABELS,
  TAKUHAI_CRAWLER_NYAA_LABELS,
  TAKUHAI_CRAWLER_PORT,
  TAKUHAI_HTTP_PORT,
  TAKUHAI_LABELS,
} from "../../constants.js";

export class TakuhaiService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "takuhai" },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "takuhai-service-pods", { labels: TAKUHAI_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: TAKUHAI_HTTP_PORT }],
    });
  }
}

export class TakuhaiCrawlerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "takuhai-crawler-dmhy" },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "crawler-dmhy-service-pods", { labels: TAKUHAI_CRAWLER_DMHY_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: TAKUHAI_CRAWLER_PORT }],
    });
  }
}

export class TakuhaiNyaaCrawlerService extends Service {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "takuhai-crawler-nyaa" },
      type: ServiceType.CLUSTER_IP,
      selector: Pods.select(scope, "crawler-nyaa-service-pods", { labels: TAKUHAI_CRAWLER_NYAA_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: TAKUHAI_CRAWLER_PORT }],
    });
  }
}
