import type { HelmProps } from "cdk8s";

import { prefer, Scheduling, workers } from "@k2/cdk-lib";

import { CustomDns } from "./custom-dns.js";

const WATCHED_RESOURCES = ["Ingress", "Service", "HTTPRoute"];
const PARENT_DOMAIN = "wyvernzora.io";

export interface K8sGatewayValuesProps {
  readonly apexDomain: string;
  readonly customDns: CustomDns;
  readonly publicDnsServers: string[];
  readonly serviceClusterIp: string;
}

export function k8sGatewayValues(props: K8sGatewayValuesProps): HelmProps["values"] {
  const scheduling = Scheduling.profile(prefer(workers()));
  return {
    replicaCount: 2,
    priorityClassName: "system-cluster-critical",
    tolerations: scheduling.tolerations,
    affinity: scheduling.affinity,
    serviceType: "ClusterIP",
    service: {
      clusterIP: props.serviceClusterIp,
      annotations: { "prometheus.io/scrape": "true", "prometheus.io/port": "9153" },
    },
    securityContext: { runAsUser: 1000 },
    resources: null,
    deployment: { dnsPolicy: "ClusterFirst" },
    rollingUpdate: { maxUnavailable: 0 },
    readinessProbe: { initialDelaySeconds: 10, failureThreshold: 5 },
    servers: [gatewayServer(props)],
  };
}

function gatewayServer(props: K8sGatewayValuesProps) {
  return {
    zones: [{ zone: ".", use_tcp: true }],
    port: 1053,
    servicePort: 53,
    plugins: [
      plugin("k8s_gateway", {
        parameters: watchedDomains(props.apexDomain).join(" "),
        configBlock: [
          "apex k8s-gateway.blocky",
          "ttl 60",
          `resources ${WATCHED_RESOURCES.join(" ")}`,
          "fallthrough",
        ].join("\n"),
      }),
      ...extraZonePlugins(props),
    ],
  };
}

function watchedDomains(apexDomain: string): string[] {
  return [...new Set([apexDomain, PARENT_DOMAIN])];
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
