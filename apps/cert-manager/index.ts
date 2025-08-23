import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import Reflector from "./components/reflector.js";
import CertManager from "./components/cert-manager.js";

export * as acmecrd from "./crds/acme.cert-manager.io.js";
export * as crd from "./crds/cert-manager.io.js";

/* Export higher level constructs */
export * from "./lib/issuer.js";
export * from "./lib/certificate.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "k2-core");
  Reflector.create(app);
  CertManager.create(app);
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "cert-manager", { namespace: "k2-core" });
};
