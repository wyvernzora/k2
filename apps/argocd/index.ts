import type { AppResourceFunc } from "@k2/cdk-lib";

import { ArgoCD } from "./components/argocd/index.js";

export * from "./lib/argo-application.js";

export const createAppResources: AppResourceFunc = app => {
  new ArgoCD(app, "argocd");
};
