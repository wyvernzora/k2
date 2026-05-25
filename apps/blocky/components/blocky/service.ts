import { k8s } from "cdk8s-plus-32";
import type { Construct } from "constructs";

export interface BlockyServiceProps {
  readonly loadBalancerIp: string;
  readonly selector: Record<string, string>;
}

export class BlockyService extends k8s.KubeService {
  public constructor(scope: Construct, id: string, props: BlockyServiceProps) {
    super(scope, id, {
      metadata: {
        name: "blocky",
        annotations: {
          "lbipam.cilium.io/ips": props.loadBalancerIp,
        },
      },
      spec: {
        type: "LoadBalancer",
        externalTrafficPolicy: "Local",
        selector: props.selector,
        ports: [
          { name: "dns-udp", protocol: "UDP", port: 53 },
          { name: "dns-tcp", protocol: "TCP", port: 53 },
        ],
      },
    });
  }
}
