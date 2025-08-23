import { AppResourceFunc, ArgoCDResourceFunc, HelmChartV1 } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import * as Auth from "@k2/auth";

/* Export raw CRDs */
export * as crd from "./crds/longhorn.io.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new HelmChartV1(app, "longhorn", {
    namespace: "k2-storage",
    chart: "helm:https://charts.longhorn.io/longhorn@1.9.1",
    values: {
      ingress: {
        enabled: true,
        host: "lh.wyvernzora.io",
        annotations: {
          ...Auth.MiddlewareAnnotation,
        },
      },
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "longhorn", { namespace: "k2-storage" });
};
