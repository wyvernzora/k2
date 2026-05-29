import type { AppResourceFunc } from "@k2/cdk-lib";

import { Longhorn } from "./components/longhorn.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * as crd from "./lib/crd.js";

export const createAppResources: AppResourceFunc = app => {
  new Longhorn(app, "longhorn");
  new NetworkPolicy(app, "network-policy");
};
