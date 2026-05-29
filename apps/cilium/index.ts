import type { AppResourceFunc } from "@k2/cdk-lib";

import { Cilium } from "./components/cilium/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * from "./lib/netpol/index.js";

export const createAppResources: AppResourceFunc = app => {
  new Cilium(app, "cilium");
  new NetworkPolicy(app, "network-policy");
};
