import type { AppResourceFunc } from "@k2/cdk-lib";

import { NfsCsi } from "./components/nfs-csi/index.js";

export const createAppResources: AppResourceFunc = app => {
  new NfsCsi(app, "nfs-csi");
};
