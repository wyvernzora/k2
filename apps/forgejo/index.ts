import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { Forgejo } from "./components/forgejo/index.js";
import { FORGEJO_HTTP_PORT, FORGEJO_LABELS, FORGEJO_SSH_PORT } from "./constants.js";

export * from "./constants.js";

export const endpoints = {
  http(): BackendTarget {
    return { backend: forgejoEndpoint(), ports: [tcp(FORGEJO_HTTP_PORT)] };
  },

  ssh(): BackendTarget {
    return { backend: forgejoEndpoint(), ports: [tcp(FORGEJO_SSH_PORT)] };
  },
};

export const workloads = {
  forgejo(): PolicyEndpoint {
    return forgejoEndpoint();
  },
};

function forgejoEndpoint(): PolicyEndpoint {
  return endpoint("forgejo", FORGEJO_LABELS, "forgejo");
}

export const createAppResources: AppResourceFunc = app => {
  new Forgejo(app, "forgejo");
  new NetworkPolicy(app, "network-policy");
};
