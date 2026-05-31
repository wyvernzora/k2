import type { AppResourceFunc } from "@k2/cdk-lib";

import { DmhyMcp } from "./components/dmhy-mcp/index.js";
import { Kura } from "./components/kura/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new DmhyMcp(app, "dmhy-mcp");
  new NetworkPolicy(app, "network-policy");
};
