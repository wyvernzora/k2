import { AppResourceFunc, ArgoCDResourceFunc } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { Reflector } from "./components/reflector";
import { CertManager } from "./components/cert-manager";
import * as AcmeCRD from "./crds/acme.cert-manager.io";
import * as MainCRD from "./crds/cert-manager.io";

/* Export raw CRDs */
export const crd = {
  ...MainCRD,
  acme: AcmeCRD,
};

/* Export higher level constructs */
export * from "./lib/issuer";
export * from "./lib/certificate";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new Reflector(app, "reflector", { namespace: "k2-core" });
  new CertManager(app, "cert-manager", {
    namespace: "k2-core",
    awsSecretId: "hxitqr6xcco7g2ne3n7m6kkoqa",
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "cert-manager", { namespace: "k2-core" });
};
