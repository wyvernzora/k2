import { App, ClusterContext, HelmCharts, Namespace, NodeAffinity } from "@k2/cdk-lib";

import { CiliumL2AnnouncementPolicy, CiliumLoadBalancerIpPool } from "../crds/cilium.io.js";

const CILIUM_OPERATOR_POD_ANTI_AFFINITY = {
  requiredDuringSchedulingIgnoredDuringExecution: [
    {
      topologyKey: "kubernetes.io/hostname",
      labelSelector: {
        matchLabels: {
          "io.cilium/app": "operator",
        },
      },
    },
  ],
};

export default {
  create(app: App) {
    const cluster = ClusterContext.of(app).cluster;
    if (!cluster.cilium) {
      throw new Error(`${cluster.id}: cilium cluster config is required to synthesize the cilium app`);
    }
    const Cilium = HelmCharts.of(app).asChart("cilium");

    const chart = new Cilium(app, "cilium", {
      ...Namespace.of(app),
      values: {
        kubeProxyReplacement: true,
        k8sServiceHost: cluster.kubernetes.api.vip,
        k8sServicePort: cluster.kubernetes.api.port,
        k8sClientRateLimit: {
          qps: 20,
          burst: 40,
        },
        l2announcements: {
          enabled: true,
        },
        ipam: {
          operator: {
            clusterPoolIPv4PodCIDRList: [cluster.kubernetes.networking.podCidr],
          },
        },
        operator: {
          replicas: 1,
          affinity: {
            podAntiAffinity: CILIUM_OPERATOR_POD_ANTI_AFFINITY,
            nodeAffinity: NodeAffinity.PREFER_NON_CONTROL_PLANE,
          },
        },
        bpf: {
          masquerade: true,
        },
      },
    });

    new CiliumLoadBalancerIpPool(chart, "default-load-balancer-pool", {
      metadata: {
        name: "default",
      },
      spec: {
        blocks: [
          {
            start: cluster.cilium.loadBalancerPool.start,
            stop: cluster.cilium.loadBalancerPool.stop,
          },
        ],
      },
    });

    new CiliumL2AnnouncementPolicy(chart, "default-l2-announcement-policy", {
      metadata: {
        name: "default",
      },
      spec: {
        loadBalancerIPs: true,
      },
    });
  },
};
