import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { ArgoCD } from "./components/argocd/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * from "./lib/argo-application.js";

const ARGOCD_NAMESPACE = "argocd";
const ARGOCD_SERVER_HTTP_PORT = 8080;
const ARGOCD_SERVER_LABELS = {
  "app.kubernetes.io/instance": "argocd",
  "app.kubernetes.io/name": "argocd-server",
};

export const endpoints = {
  http(): BackendTarget {
    return { backend: serverEndpoint(), ports: [tcp(ARGOCD_SERVER_HTTP_PORT)] };
  },
};

function serverEndpoint(): PolicyEndpoint {
  return endpoint(ARGOCD_NAMESPACE, ARGOCD_SERVER_LABELS, "argocd-server");
}

export const createAppResources: AppResourceFunc = app => {
  new ArgoCD(app, "argocd");
  new NetworkPolicy(app, "network-policy");
};
