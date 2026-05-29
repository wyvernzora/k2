import { ApiObject, JsonPatch } from "cdk8s";
import { Pods, Protocol, Service, ServiceType } from "cdk8s-plus-32";
import type { Construct } from "constructs";

export interface BlockyServiceProps {
  readonly loadBalancerIp: string;
  readonly selector: Record<string, string>;
}

export class BlockyService extends Service {
  public constructor(scope: Construct, id: string, props: BlockyServiceProps) {
    super(scope, id, {
      metadata: {
        name: "blocky",
        annotations: {
          "lbipam.cilium.io/ips": props.loadBalancerIp,
        },
      },
      type: ServiceType.LOAD_BALANCER,
      selector: Pods.select(scope, "blocky-service-pods", { labels: props.selector }),
      ports: [
        { name: "dns-udp", protocol: Protocol.UDP, port: 53 },
        { name: "dns-tcp", protocol: Protocol.TCP, port: 53 },
      ],
    });
    ApiObject.of(this).addJsonPatch(JsonPatch.add("/spec/externalTrafficPolicy", "Local"));
  }
}
