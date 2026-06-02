import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { Paperless } from "./components/paperless/index.js";

export const createAppResources: AppResourceFunc = app => {
  new Paperless(app, "paperless");
  new NetworkPolicy(app, "network-policy");
};
