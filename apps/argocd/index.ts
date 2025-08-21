/* Export raw CRDs */
import { AppResourceFunc, ArgoCDResourceFunc } from "@k2/cdk-lib";
import * as CRD from "./crds/argoproj.io";
import { ArgoCd } from "./components/argocd";
import { ContinuousDeployment } from "./lib/cd";
export const crd = {
  ...CRD,
};

/* Export higher level constructs */
export * from "./lib/cd";
export * from "./lib/context";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new ArgoCd(app, "argocd", {
    subdomain: "deploy",
    namespace: "k2-core",
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "argocd", { namespace: "k2-core" });
};
