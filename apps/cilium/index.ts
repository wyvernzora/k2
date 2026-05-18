import { AppResourceFunc, ArgoCDResourceFunc, defineDeployment, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import Cilium from "./components/cilium.js";
export * as crd from "./crds/cilium.io.js";

export const deployment = defineDeployment({
  targets: {
    legacy: false,
    v3: {
      enabled: true,
      bootstrap: true,
      argo: true,
    },
  },
});

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "cilium");
  Cilium.create(app);
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "cilium", { namespace: "cilium" });
};
