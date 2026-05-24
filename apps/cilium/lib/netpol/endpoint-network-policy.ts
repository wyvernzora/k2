import type { Construct } from "constructs";

import { CiliumClusterwideNetworkPolicy } from "../../crds/cilium.io.js";

import type { EndpointNetworkPolicyProps } from "./types.js";
import { endpointPolicyName } from "./names.js";
import { clusterwideEgressRule, clusterwideIngressRule } from "./rules.js";
import { endpointSelector } from "./selectors.js";

export class EndpointNetworkPolicy extends CiliumClusterwideNetworkPolicy {
  public constructor(scope: Construct, id: string, props: EndpointNetworkPolicyProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? endpointPolicyName(id, props.endpoint),
      },
      specs: [
        {
          description: props.description,
          endpointSelector: endpointSelector(props.endpoint),
          ingress: props.ingress?.map(clusterwideIngressRule),
          egress: props.egress?.map(clusterwideEgressRule),
        },
      ],
    });
  }
}
