import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { PocketId } from "./components/pocket-id/index.js";

export * from "./lib/constants.js";

export const createAppResources: AppResourceFunc = app => {
  new PocketId(app, "pocket-id");
  new NetworkPolicy(app, "network-policy");
};
