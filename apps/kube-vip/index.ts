import { AppResourceFunc, ArgoCDResourceFunc, defineDeployment, HelmCharts, Namespace, Toleration } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

export const deployment = defineDeployment({
  targets: {
    legacy: true,
    v3: {
      enabled: true,
      bootstrap: true,
      argo: true,
    },
  },
});

function namespaceForTarget(target: Parameters<AppResourceFunc>[1]["target"]): string {
  return target === "v3" ? "kube-vip" : "k2-network";
}

function apiVipForTarget(target: Parameters<AppResourceFunc>[1]["target"]): string {
  return target === "v3" ? "10.10.9.1" : "10.10.8.2";
}

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = (app, ctx) => {
  app.use(Namespace, namespaceForTarget(ctx.target));
  const helm = HelmCharts.of(app);
  const KubeVip = helm.asChart("kube-vip");

  new KubeVip(app, "kube-vip", {
    ...Namespace.of(app),
    values: {
      config: {
        address: apiVipForTarget(ctx.target),
      },
      env: {
        cp_enable: "true",
        svc_enable: "false",
        vip_leaderelection: "true",
      },
      tolerations: Toleration.ALLOW_CONTROL_PLANE,
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
export const createArgoCdResources: ArgoCDResourceFunc = (chart, ctx) => {
  new ContinuousDeployment(chart, "kube-vip", { namespace: namespaceForTarget(ctx.target) });
};
