import type { Construct } from "constructs";

import { oci } from "@k2/cdk-lib";

import { ProxyClass, type ProxyClassSpec } from "../crds/tailscale.com.js";

export interface TailscaleRouterProxyClassProps {
  readonly name: string;
}

export class TailscaleRouterProxyClass extends ProxyClass {
  public constructor(scope: Construct, id: string, props: TailscaleRouterProxyClassProps) {
    super(scope, id, {
      metadata: { name: props.name },
      spec: routerProxyClassSpec(),
    });
  }
}

function routerProxyClassSpec(): ProxyClassSpec {
  return {
    statefulSet: {
      pod: {
        tailscaleContainer: tailscaleContainerSpec(),
      },
    },
  };
}

function tailscaleContainerSpec(): NonNullable<
  NonNullable<NonNullable<ProxyClassSpec["statefulSet"]>["pod"]>["tailscaleContainer"]
> {
  return {
    // The released proxy includes Go's eMudhra fix ahead of the operator chart.
    image: oci`tailscale/tailscale:v1.104.2`,
    env: [
      {
        name: "TS_DEBUG_FIREWALL_MODE",
        value: "iptables",
      },
    ],
  };
}
