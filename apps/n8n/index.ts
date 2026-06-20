import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { N8N } from "./components/n8n/index.js";

export const createAppResources: AppResourceFunc = app => {
  new N8N(app, "n8n");
  new NetworkPolicy(app, "network-policy");
};
