import type { AppResourceFunc } from "@k2/cdk-lib";
import { endpoint, tcp, type BackendTarget } from "@k2/cilium";

import { Metube } from "./components/metube.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { METUBE_HTTP_PORT, METUBE_LABELS } from "./constants.js";

export const endpoints = {
  web(): BackendTarget {
    return { backend: endpoint("metube", METUBE_LABELS, "metube"), ports: [tcp(METUBE_HTTP_PORT)] };
  },
};

export const createAppResources: AppResourceFunc = app => {
  new Metube(app, "metube");
  new NetworkPolicy(app, "network-policy");
};
