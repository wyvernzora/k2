import type { AppResourceFunc } from "@k2/cdk-lib";

import { Homer } from "./components/homer/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export const createAppResources: AppResourceFunc = app => {
  new Homer(app, "homer");
  new NetworkPolicy(app, "network-policy");
};
