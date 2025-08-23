import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import Authelia from "./components/authelia.js";
import Glauth from "./components/glauth/index.js";

/* Export higher level constructs */
export * from "./lib/ingress.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "auth");
  Authelia.create(app);
  Glauth.create(app);
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "auth");
};
