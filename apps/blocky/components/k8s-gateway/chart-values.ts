import type { HelmProps } from "cdk8s";

import { Scheduling } from "@k2/cdk-lib";

import { CustomDns } from "./custom-dns.js";

const WATCHED_RESOURCES = ["Ingress", "Service", "HTTPRoute"];

export interface K8sGatewayValuesProps {
  readonly apexDomain: string;
  readonly customDns: CustomDns;
  readonly publicDnsServers: string[];
  readonly serviceClusterIp: string;
}

export function k8sGatewayValues(props: K8sGatewayValuesProps): HelmProps["values"] {
  const scheduling = Scheduling.workersPreferred();
  return {
    domain: props.apexDomain,
    ttl: 60,
    replicaCount: 2,
    watchedResources: WATCHED_RESOURCES,
    priorityClassName: "system-cluster-critical",
    tolerations: scheduling.tolerations,
    affinity: scheduling.affinity,
    fallthrough: {
      enabled: true,
    },
    service: {
      type: "ClusterIP",
      clusterIP: props.serviceClusterIp,
      useTcp: true,
    },
    extraZonePlugins: extraZonePlugins(props),
  };
}

function extraZonePlugins(props: K8sGatewayValuesProps): K8sGatewayPlugin[] {
  return [
    plugin("log"),
    plugin("errors"),
    plugin("hosts", { configBlock: props.customDns.toHostsPluginBlock() }),
    plugin("health", { configBlock: "lameduck 5s" }),
    plugin("ready"),
    plugin("prometheus", { parameters: "0.0.0.0:9153" }),
    plugin("forward", { parameters: `. ${props.publicDnsServers.join(" ")}` }),
    plugin("cache", { parameters: "30" }),
    plugin("loop"),
    plugin("reload"),
    plugin("loadbalance"),
  ];
}

interface K8sGatewayPlugin {
  readonly name: string;
  readonly parameters?: string;
  readonly configBlock?: string;
}

function plugin(name: string, options: Omit<K8sGatewayPlugin, "name"> = {}): K8sGatewayPlugin {
  return {
    name,
    ...options,
  };
}
