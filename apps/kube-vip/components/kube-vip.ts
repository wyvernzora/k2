import type { Construct } from "constructs";

import { ClusterContext, HelmCharts, K2Chart, Scheduling } from "@k2/cdk-lib";

export class KubeVip extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const cluster = ClusterContext.of(this).config;
    const scheduling = Scheduling.controlPlane();

    HelmCharts.of(this).asChart(this, "kube-vip", "kube-vip", {
      config: {
        address: cluster.kubernetes.api,
      },
      env: {
        cp_enable: "true",
        KUBERNETES_SERVICE_HOST: "127.0.0.1",
        KUBERNETES_SERVICE_PORT: "6443",
        svc_enable: "false",
        vip_leaderelection: "true",
      },
      resources: {
        limits: {
          cpu: "200m",
          memory: "200Mi",
        },
        requests: {
          cpu: "50m",
          memory: "100Mi",
        },
      },
      tolerations: scheduling.tolerations,
      affinity: scheduling.affinity,
    });
  }
}
