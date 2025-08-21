import { AppResourceFunc, ArgoCDResourceFunc } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import { Authelia } from "./components/authelia";

/* Export higher level constructs */
export * from "./lib/ingress";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new Authelia(app, "authelia");
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "authelia", { namespace: "k2-auth" });
};
