import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { Homer } from "./components/homer/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { HOMER_HTTP_PORT, HOMER_LABELS } from "./constants.js";

const HOMER_NAMESPACE = "homer";

export const endpoints = {
  http(): BackendTarget {
    return { backend: homerEndpoint(), ports: [tcp(HOMER_HTTP_PORT)] };
  },
};

function homerEndpoint(): PolicyEndpoint {
  return endpoint(HOMER_NAMESPACE, HOMER_LABELS, "homer");
}

export const createAppResources: AppResourceFunc = app => {
  new Homer(app, "homer");
  new NetworkPolicy(app, "network-policy");
};
