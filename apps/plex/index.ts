import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { Plex } from "./components/plex/index.js";
import { PLEX_LABELS } from "./constants.js";

const PLEX_NAMESPACE = "plex";

export const workloads = {
  plex(): PolicyEndpoint {
    return endpoint(PLEX_NAMESPACE, PLEX_LABELS, "plex");
  },
};

export const createAppResources: AppResourceFunc = app => {
  new Plex(app, "plex");
  new NetworkPolicy(app, "network-policy");
};
