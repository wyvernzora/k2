import type { AppResourceFunc } from "@k2/cdk-lib";

import type { PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { PomeriumController } from "./components/controller/index.js";
import { PomeriumGlobalConfig } from "./components/global-config.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { POMERIUM_LABELS, POMERIUM_NAMESPACE } from "./constants.js";

export * as crd from "./lib/crd.js";
export * from "./constants.js";
export * from "./lib/ingress.js";
export * from "./lib/network-policy.js";
export * from "./lib/policy.js";

export const workloads = {
  proxy(): PolicyEndpoint {
    return {
      name: "pomerium",
      namespace: POMERIUM_NAMESPACE,
      labels: POMERIUM_LABELS,
    };
  },
};

export const createAppResources: AppResourceFunc = app => {
  new PomeriumController(app, "pomerium");
  new PomeriumGlobalConfig(app, "global-config");
  new NetworkPolicy(app, "network-policy");
};
