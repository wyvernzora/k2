import type { AppResourceFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { Qbittorrent } from "./components/qbittorrent/index.js";

export const createAppResources: AppResourceFunc = app => {
  new Qbittorrent(app, "qbittorrent");
  new NetworkPolicy(app, "network-policy");
};
