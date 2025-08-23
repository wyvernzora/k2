import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, HelmCharts, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import * as Auth from "@k2/auth";

/* Export raw CRDs */
export * as crd from "./crds/longhorn.io.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "k2-storage");
  const Longhorn = HelmCharts.of(app).asChart("longhorn");

  new Longhorn(app, "longhorn", {
    ...Namespace.of(app),
    values: {
      ingress: {
        enabled: true,
        host: ApexDomain.of(app).subdomain("lh"),
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
