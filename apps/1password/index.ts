import { AppResourceFunc, ArgoCDResourceFunc, HelmCharts, Toleration } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

/* Export raw CRDs */
import * as OnePasswordCRD from "./crds/onepassword.com";
export const crd = {
  ...OnePasswordCRD,
};

/* Export higher level constructs */
export * from "./lib/item";
export * from "./lib/context";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const helm = HelmCharts.of(app);
  const OnePassword = helm.asChart("1password-connect");

  new OnePassword(app, "1password", {
    namespace: "k2-core",
    values: {
      connect: {
        tolerations: [...Toleration.ALLOW_CRITICAL_ADDONS_ONLY, ...Toleration.ALLOW_CONTROL_PLANE],
      },
      operator: {
        create: true,
        tolerations: [...Toleration.ALLOW_CRITICAL_ADDONS_ONLY, ...Toleration.ALLOW_CONTROL_PLANE],
      },
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "1password", { namespace: "k2-core" });
};
