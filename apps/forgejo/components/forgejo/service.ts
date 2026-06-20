import { Pods, Protocol } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { LoadBalancerService } from "@k2/cilium";

import {
  FORGEJO_ALLOW_VLANS,
  FORGEJO_HOST,
  FORGEJO_HTTPS_PORT,
  FORGEJO_LABELS,
  FORGEJO_SSH_PORT,
} from "../../constants.js";

const HTTPS_PORT = 443;
const SERVICE_NAME = "forgejo";
const LOAD_BALANCER_IP = "10.10.13.3";

export class ForgejoService extends LoadBalancerService {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      name: SERVICE_NAME,
      loadBalancerIp: LOAD_BALANCER_IP,
      allowVlans: FORGEJO_ALLOW_VLANS,
      annotations: {
        "external-dns.alpha.kubernetes.io/hostname": FORGEJO_HOST,
      },
      externalTrafficPolicy: "Cluster",
      selector: Pods.select(scope, "forgejo-service-pods", { labels: FORGEJO_LABELS }),
      ports: [
        {
          name: "https",
          protocol: Protocol.TCP,
          port: HTTPS_PORT,
          targetPort: FORGEJO_HTTPS_PORT,
        },
        {
          name: "ssh",
          protocol: Protocol.TCP,
          port: FORGEJO_SSH_PORT,
          targetPort: FORGEJO_SSH_PORT,
        },
      ],
    });
  }
}
