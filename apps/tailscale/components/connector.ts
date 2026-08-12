import type { Construct } from "constructs";

import { ClusterContext, K2Chart } from "@k2/cdk-lib";

import { Connector } from "../crds/tailscale.com.js";

import { TailscaleRouterProxyClass } from "./proxy-class.js";

const CONNECTOR_NAME = "k2-router";
const PROXY_CLASS_NAME = "k2-router-iptables";
const INFRASTRUCTURE_VLAN_NAME = "infrastructure";

export class TailscaleConnector extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new TailscaleRouterProxyClass(this, "proxy-class", {
      name: PROXY_CLASS_NAME,
    });

    new Connector(this, "k2-router", {
      metadata: { name: CONNECTOR_NAME },
      spec: {
        hostname: CONNECTOR_NAME,
        proxyClass: PROXY_CLASS_NAME,
        subnetRouter: {
          advertiseRoutes: subnetRoutes(this),
        },
      },
    });
  }
}

function subnetRoutes(scope: Construct): string[] {
  const vlan = ClusterContext.of(scope).config.network.vlans.find(
    candidate => candidate.name === INFRASTRUCTURE_VLAN_NAME,
  );
  if (vlan === undefined) {
    throw new Error(`Tailscale connector requires network.vlans entry ${INFRASTRUCTURE_VLAN_NAME}`);
  }
  return [vlan.cidr];
}
