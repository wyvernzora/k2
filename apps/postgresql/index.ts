import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import CloudNativePG from "./components/cloud-native-pg.js";
import DbClaimOperator from "./components/dbclaim-operator.js";
import NexusCluster from "./components/nexus-cluster.js";

export * as crd from "./crds/postgresql.cnpg.io.js";
export * from "./lib/nexus.js";
export * from "./lib/database-claim.js";
export * from "./lib/role-claim.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "postgresql");
  CloudNativePG.create(app);
  DbClaimOperator.create(app);
  NexusCluster.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "postgresql");
};
