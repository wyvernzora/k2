import type { AppResourceFunc } from "@k2/cdk-lib";

import { KubeVip } from "./components/kube-vip.js";

export const createAppResources: AppResourceFunc = app => {
  new KubeVip(app, "kube-vip");
};
