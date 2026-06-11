import { ApiObject, JsonPatch, type ApiObjectMetadata } from "cdk8s";
import { Service, ServiceType, type IPodSelector, type ServicePort } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { vlanCidrs } from "./vlans.js";

export interface LoadBalancerServiceProps {
  readonly name: string;
  readonly loadBalancerIp: string;
  readonly selector: IPodSelector;
  readonly ports: ServicePort[];
  readonly allowVlans?: string[];
  readonly annotations?: ApiObjectMetadata["annotations"];
  readonly externalTrafficPolicy?: "Cluster" | "Local";
}

export class LoadBalancerService extends Service {
  public readonly allowedCidrs: string[];

  public constructor(scope: Construct, id: string, props: LoadBalancerServiceProps) {
    const allowedCidrs = props.allowVlans === undefined ? [] : vlanCidrs(scope, props.allowVlans);
    super(scope, id, {
      metadata: {
        name: props.name,
        annotations: {
          ...props.annotations,
          "lbipam.cilium.io/ips": props.loadBalancerIp,
        },
      },
      type: ServiceType.LOAD_BALANCER,
      selector: props.selector,
      ports: props.ports,
      loadBalancerSourceRanges: allowedCidrs.length > 0 ? allowedCidrs : undefined,
    });

    this.allowedCidrs = allowedCidrs;
    ApiObject.of(this).addJsonPatch(
      JsonPatch.add("/spec/externalTrafficPolicy", props.externalTrafficPolicy ?? "Local"),
    );
  }
}
