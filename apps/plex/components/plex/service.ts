import { Pods, Protocol } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { LoadBalancerService } from "@k2/cilium";

import { PLEX_ALLOW_VLANS, PLEX_CADDY_PORT, PLEX_HTTPS_PORT, PLEX_LABELS, PLEX_SERVICE_NAME } from "./labels.js";

const PLEX_LOAD_BALANCER_IP = "10.10.13.2";

export class PlexService extends LoadBalancerService {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      name: PLEX_SERVICE_NAME,
      loadBalancerIp: PLEX_LOAD_BALANCER_IP,
      allowVlans: PLEX_ALLOW_VLANS,
      externalTrafficPolicy: "Cluster",
      selector: Pods.select(scope, "plex-service-pods", { labels: PLEX_LABELS }),
      ports: [
        {
          name: "https",
          protocol: Protocol.TCP,
          port: PLEX_HTTPS_PORT,
          targetPort: PLEX_CADDY_PORT,
        },
      ],
    });
  }
}
