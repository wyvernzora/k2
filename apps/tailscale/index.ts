import type { AppResourceFunc } from "@k2/cdk-lib";
import { endpoint, tcp, udp, type CidrTarget, type PolicyEndpoint, type PortSpec } from "@k2/cilium";

import { TailscaleConnector } from "./components/connector.js";
import { TailscaleOperator } from "./components/tailscale-operator.js";

const TAILSCALE_NAMESPACE = "tailscale";
const CONNECTOR_NAME = "k2-router";
const TAILNET_IPV4_CIDRS = ["100.64.0.0/10"];
const DNS_PORTS = [udp(53), tcp(53)];

export const endpoints = {
  tailnetClients(...ports: PortSpec[]): CidrTarget {
    return tailnetClients(...ports);
  },
  tailnetDnsClients(): CidrTarget {
    return tailnetClients(...DNS_PORTS);
  },
};

function tailnetClients(...ports: PortSpec[]): CidrTarget {
  return { cidrs: TAILNET_IPV4_CIDRS, ports };
}

export const workloads = {
  router(): PolicyEndpoint {
    return endpoint(
      TAILSCALE_NAMESPACE,
      {
        "tailscale.com/managed": "true",
        "tailscale.com/parent-resource": CONNECTOR_NAME,
        "tailscale.com/parent-resource-type": "connector",
      },
      CONNECTOR_NAME,
    );
  },
};

export const createAppResources: AppResourceFunc = app => {
  new TailscaleOperator(app, "tailscale-operator");
  new TailscaleConnector(app, "connector");
};
