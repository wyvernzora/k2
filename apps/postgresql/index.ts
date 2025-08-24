import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import CloudNativePG from "./components/cloud-native-pg.js";

export * as crd from "./crds/postgresql.cnpg.io.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "postgresql");
  CloudNativePG.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "postgresql");
};
