import { AppResourceFunc, ArgoCDResourceFunc } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import { Authelia } from "./components/authelia";
import { Glauth } from "./components/glauth";

/* Export higher level constructs */
export * from "./lib/ingress";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new Authelia(app, "authelia");
  new Glauth(app, "glauth");
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "auth", { namespace: "auth" });
};
