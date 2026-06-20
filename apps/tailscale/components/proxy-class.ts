import type { Construct } from "constructs";

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
    env: [
      {
        name: "TS_DEBUG_FIREWALL_MODE",
        value: "iptables",
      },
    ],
  };
}
