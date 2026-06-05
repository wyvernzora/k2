import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type PrivateConnectionTarget } from "../cilium/lib/netpol/index.js";

import { CNPG } from "./components/cnpg/index.js";
import { DbClaimOperator } from "./components/dbclaim-operator/index.js";
import { NetworkPolicies } from "./components/network-policies.js";
import { NexusCluster } from "./components/nexus-cluster.js";
import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "./lib/nexus.js";

export * as crd from "./lib/crd.js";
export * from "./lib/database-claim.js";
export * from "./lib/nexus.js";
export * from "./lib/role-claim.js";

const POSTGRES_PORT = 5432;

export const endpoints = {
  nexus(): PrivateConnectionTarget {
    return {
      to: endpoint(NEXUS_CLUSTER_NAMESPACE, { "cnpg.io/cluster": NEXUS_CLUSTER_NAME }, "nexus-postgresql"),
      ports: [tcp(POSTGRES_PORT)],
    };
  },
};

export const createAppResources: AppResourceFunc = app => {
  new CNPG(app, "cnpg");
  new DbClaimOperator(app, "dbclaim-operator");
  new NexusCluster(app, "nexus-cluster");
  new NetworkPolicies(app, "network-policies");
};
