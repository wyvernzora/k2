import type { Construct } from "constructs";

import { ClusterContext, HelmCharts, K2Chart, Scheduling } from "@k2/cdk-lib";

import { CiliumL2AnnouncementPolicy, CiliumLoadBalancerIpPool } from "../../crds/cilium.io.js";

import { HubbleIngress } from "./hubble-ingress.js";

const CILIUM_OPERATOR_LABEL_SELECTOR = {
  matchLabels: {
    "io.cilium/app": "operator",
  },
};

const CILIUM_OPERATOR_POD_ANTI_AFFINITY = {
  requiredDuringSchedulingIgnoredDuringExecution: [
    {
      topologyKey: "kubernetes.io/hostname",
      labelSelector: CILIUM_OPERATOR_LABEL_SELECTOR,
    },
  ],
};

export class Cilium extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const cluster = ClusterContext.of(this).config;

    HelmCharts.of(this).asChart(this, "cilium", "cilium", {
      kubeProxyReplacement: true,
      k8sServiceHost: cluster.kubernetes.api,
      k8sServicePort: 6443,
      k8sClientRateLimit: {
        qps: 20,
        burst: 40,
      },
      l2announcements: {
        enabled: true,
      },
      ipam: {
        operator: {
          clusterPoolIPv4PodCIDRList: [cluster.kubernetes.subnets.pods],
        },
      },
      operator: {
        replicas: 1,
        affinity: {
          podAntiAffinity: CILIUM_OPERATOR_POD_ANTI_AFFINITY,
          nodeAffinity: Scheduling.controlPlanePreferred().affinity!.nodeAffinity,
        },
      },
      bpf: {
        masquerade: true,
      },
      // Hubble = Cilium's observability layer. Relay aggregates per-node
      // flow data; UI consumes Relay. Co-located in the cilium namespace
      // (chart default) — they're adjuncts of cilium itself, not a
      // standalone product.
      hubble: {
        relay: { enabled: true },
        ui: { enabled: true },
      },
    });

    for (const pool of cluster.loadBalancerPools) {
      new CiliumLoadBalancerIpPool(this, `lb-pool-${pool.name}`, {
        metadata: { name: pool.name },
        spec: {
          blocks: [{ cidr: pool.cidr }],
        },
      });
    }

    new CiliumL2AnnouncementPolicy(this, "l2-announce", {
      metadata: { name: "default" },
      spec: {
        loadBalancerIPs: true,
      },
    });

    new HubbleIngress(this, "hubble-ingress");
  }
}
