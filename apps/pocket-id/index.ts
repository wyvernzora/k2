import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { PocketId } from "./components/pocket-id/index.js";
import { POCKET_ID_HTTP_PORT, POCKET_ID_LABELS, POCKET_ID_NAMESPACE } from "./constants.js";

export * from "./constants.js";

export const endpoints = {
  http(): BackendTarget {
    return { backend: pocketIdEndpoint(), ports: [tcp(POCKET_ID_HTTP_PORT)] };
  },
};

function pocketIdEndpoint(): PolicyEndpoint {
  return endpoint(POCKET_ID_NAMESPACE, POCKET_ID_LABELS, "pocket-id");
}

export const createAppResources: AppResourceFunc = app => {
  new PocketId(app, "pocket-id");
  new NetworkPolicy(app, "network-policy");
};
