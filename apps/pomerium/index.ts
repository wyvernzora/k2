import type { AppResourceFunc } from "@k2/cdk-lib";

import { PomeriumController } from "./components/controller/index.js";
import { PomeriumGlobalConfig } from "./components/global-config.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * as crd from "./lib/crd.js";
export * from "./lib/constants.js";
export * from "./lib/ingress.js";
export * from "./lib/network-policy.js";
export * from "./lib/policy.js";

export const createAppResources: AppResourceFunc = app => {
  new PomeriumController(app, "pomerium");
  new PomeriumGlobalConfig(app, "global-config");
  new NetworkPolicy(app, "network-policy");
};
