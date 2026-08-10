import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { SnapshotController } from "./components/snapshot-controller.js";

export const createAppResources: AppResourceFunc = app => {
  new SnapshotController(app, "snapshot-controller");
  new NetworkPolicy(app, "network-policy");
};
