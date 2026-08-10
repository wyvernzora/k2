import type { AppResourceFunc } from "@k2/cdk-lib";

import { DemocraticCsi } from "./components/democratic-csi/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export const createAppResources: AppResourceFunc = app => {
  new DemocraticCsi(app, "democratic-csi");
  new NetworkPolicy(app, "network-policy");
};
