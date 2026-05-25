import type { AppResourceFunc } from "@k2/cdk-lib";

import { Traefik } from "./components/traefik.js";

export * as crd from "./lib/crd.js";
export * from "./lib/network-policy.js";

export const createAppResources: AppResourceFunc = app => {
  new Traefik(app, "traefik");
};
