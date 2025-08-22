import { AppResourceFunc, ArgoCDResourceFunc, HelmChartV1 } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new HelmChartV1(app, "kube-vip", {
    namespace: "k2-network",
    chart: "helm:https://kube-vip.github.io/helm-charts/kube-vip@0.8.0",
    values: {
      config: {
        address: "10.10.8.2",
      },
      env: {
        cp_enable: "true",
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
      affinity: {
        nodeAffinity: {
          requiredDuringSchedulingIgnoredDuringExecution: {
            nodeSelectorTerms: [
              {
                matchExpressions: [
                  {
                    key: "node-role.kubernetes.io/master",
                    operator: "Exists",
                  },
                ],
              },
              {
                matchExpressions: [
                  {
                    key: "node-role.kubernetes.io/control-plane",
                    operator: "Exists",
                  },
                ],
              },
            ],
          },
        },
      },
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "kube-vip", { namespace: "k2-network" });
};
