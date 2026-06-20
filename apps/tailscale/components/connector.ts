import type { Construct } from "constructs";

import { ClusterContext, K2Chart } from "@k2/cdk-lib";

import { Connector } from "../crds/tailscale.com.js";

import { TailscaleRouterProxyClass } from "./proxy-class.js";

const CONNECTOR_NAME = "k2-router";
const PROXY_CLASS_NAME = "k2-router-iptables";
const BLOCKY_POOL_NAME = "blocky";
const K2_FRONT_DOOR_POOL_NAME = "privileged";

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
  const pools = ClusterContext.of(scope).config.loadBalancerPools;
  return [singleAddressPool(pools, BLOCKY_POOL_NAME), cidrPool(pools, K2_FRONT_DOOR_POOL_NAME)];
}

function singleAddressPool(pools: { name: string; cidr: string }[], name: string): string {
  const cidr = cidrPool(pools, name);
  if (!cidr.endsWith("/32")) {
    throw new Error(`Tailscale connector route ${name} must be a single-address /32 pool`);
  }
  return cidr;
}

function cidrPool(pools: { name: string; cidr: string }[], name: string): string {
  const pool = pools.find(candidate => candidate.name === name);
  if (pool === undefined) {
    throw new Error(`Tailscale connector requires loadBalancerPools entry ${name}`);
  }
  return pool.cidr;
}
