import type { Construct } from "constructs";

import { CiliumClusterwideNetworkPolicy } from "../../crds/cilium.io.js";

import { connectionPolicyName, privateConnectionDescription } from "./names.js";
import { clusterwideEgressPorts, clusterwideIngressPorts } from "./ports.js";
import { endpointSelector } from "./selectors.js";
import type { PrivateConnectionProps } from "./types.js";

export class PrivateConnection extends CiliumClusterwideNetworkPolicy {
  public constructor(scope: Construct, id: string, props: PrivateConnectionProps) {
    super(scope, id, {
      metadata: {
        name: props.name ?? connectionPolicyName(id, props.from, props.to),
      },
      specs: [
        {
          description: props.description ?? privateConnectionDescription(props.from, props.to),
          endpointSelector: endpointSelector(props.from),
          egress: [
            {
              toEndpoints: [endpointSelector(props.to)],
              toPorts: clusterwideEgressPorts(props.ports),
            },
          ],
        },
        {
          description: props.description ?? privateConnectionDescription(props.from, props.to),
          endpointSelector: endpointSelector(props.to),
          ingress: [
            {
              fromEndpoints: [endpointSelector(props.from)],
              toPorts: clusterwideIngressPorts(props.ports),
            },
          ],
        },
      ],
    });
  }
}
