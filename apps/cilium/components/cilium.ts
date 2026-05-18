import { App, HelmCharts, Namespace, NodeAffinity } from "@k2/cdk-lib";

import { CiliumL2AnnouncementPolicy, CiliumLoadBalancerIpPool } from "../crds/cilium.io.js";

const KUBERNETES_API_VIP = "10.10.9.1";
const LOAD_BALANCER_POOL_START = "10.10.9.16";
const LOAD_BALANCER_POOL_STOP = "10.10.9.255";
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
    const Cilium = HelmCharts.of(app).asChart("cilium");

    const chart = new Cilium(app, "cilium", {
      ...Namespace.of(app),
      values: {
        kubeProxyReplacement: true,
        k8sServiceHost: KUBERNETES_API_VIP,
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
            clusterPoolIPv4PodCIDRList: ["10.42.0.0/16"],
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
            start: LOAD_BALANCER_POOL_START,
            stop: LOAD_BALANCER_POOL_STOP,
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
