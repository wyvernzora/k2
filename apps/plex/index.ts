import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { Plex } from "./components/plex/index.js";

export const createAppResources: AppResourceFunc = app => {
  new Plex(app, "plex");
  new NetworkPolicy(app, "network-policy");
};
