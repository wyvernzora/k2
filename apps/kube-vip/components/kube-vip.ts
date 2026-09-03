import type { Construct } from "constructs";

import { ClusterContext, controlPlane, HelmCharts, K2Chart, only, Scheduling } from "@k2/cdk-lib";

export class KubeVip extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const cluster = ClusterContext.of(this).config;
    const scheduling = Scheduling.profile(only(controlPlane()));

    cluster.kubernetes.api.vips.forEach((vip, index) => {
      const isExistingInstance = vip.name === "kube-vip";

      HelmCharts.of(this).asChart(this, vip.name, "kube-vip", {
        nameOverride: vip.name,
        config: {
          address: vip.address,
        },
        env: {
          cp_enable: "true",
          KUBERNETES_SERVICE_HOST: "127.0.0.1",
          KUBERNETES_SERVICE_PORT: "6443",
          prometheus_server: `:${2112 + index}`,
          svc_enable: "false",
          vip_interface: vip.interface ?? "",
          vip_leaderelection: "true",
          ...(isExistingInstance
            ? {}
            : {
                instance_name: vip.name,
                vip_leasename: `${vip.name}-cp-lock`,
              }),
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
    });
  }
}
