import { AppResourceFunc, ArgoCDResourceFunc, HelmChart } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import * as Auth from "@k2/auth";

/* Export raw CRDs */
import * as CRD from "./crds/longhorn.io";
export const crd = {
  ...CRD,
};

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new HelmChart(app, "longhorn", {
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
