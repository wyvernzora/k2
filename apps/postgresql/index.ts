import type { AppResourceFunc } from "@k2/cdk-lib";

import { CNPG } from "./components/cnpg/index.js";
import { DbClaimOperator } from "./components/dbclaim-operator/index.js";
import { NetworkPolicies } from "./components/network-policies.js";
import { NexusCluster } from "./components/nexus-cluster.js";

export * as crd from "./lib/crd.js";
export * from "./lib/database-claim.js";
export * from "./lib/nexus.js";
export * from "./lib/role-claim.js";

export const createAppResources: AppResourceFunc = app => {
  new CNPG(app, "cnpg");
  new DbClaimOperator(app, "dbclaim-operator");
  new NexusCluster(app, "nexus-cluster");
  new NetworkPolicies(app, "network-policies");
};
