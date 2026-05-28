import { KubeClusterRole, KubeClusterRoleBinding } from "cdk8s-plus-32/lib/imports/k8s.js";
import type { Construct } from "constructs";

import { POMERIUM_CONTROLLER_NAME, POMERIUM_NAMESPACE } from "../../lib/constants.js";

import { clusterMetadata } from "./metadata.js";

export const GEN_SECRETS_SERVICE_ACCOUNT = "pomerium-gen-secrets";

export function createRbac(scope: Construct): void {
  new KubeClusterRole(scope, "controller-cluster-role", controllerClusterRole());
  new KubeClusterRole(scope, "gen-secrets-cluster-role", genSecretsClusterRole());
  new KubeClusterRoleBinding(scope, "controller-cluster-role-binding", clusterRoleBinding(POMERIUM_CONTROLLER_NAME));
  new KubeClusterRoleBinding(
    scope,
    "gen-secrets-cluster-role-binding",
    clusterRoleBinding(GEN_SECRETS_SERVICE_ACCOUNT),
  );
}

function controllerClusterRole() {
  return {
    metadata: clusterMetadata(POMERIUM_CONTROLLER_NAME),
    rules: [
      rbacRule([""], ["services", "endpoints", "secrets"], ["get", "list", "watch"]),
      rbacRule([""], ["services/status", "secrets/status", "endpoints/status"], ["get"]),
      rbacRule(["networking.k8s.io"], ["ingresses", "ingressclasses"], ["get", "list", "watch"]),
      rbacRule(["networking.k8s.io"], ["ingresses/status"], ["get", "patch", "update"]),
      rbacRule(["ingress.pomerium.io"], ["pomerium"], ["get", "list", "watch"]),
      rbacRule(["ingress.pomerium.io"], ["pomerium/status"], ["get", "update", "patch"]),
      rbacRule([""], ["events"], ["create", "patch"]),
    ],
  };
}

function genSecretsClusterRole() {
  return {
    metadata: clusterMetadata(GEN_SECRETS_SERVICE_ACCOUNT),
    rules: [rbacRule([""], ["secrets"], ["create", "get"])],
  };
}

function rbacRule(apiGroups: string[], resources: string[], verbs: string[]) {
  return { apiGroups, resources, verbs };
}

function clusterRoleBinding(name: string) {
  return {
    metadata: clusterMetadata(name),
    roleRef: {
      apiGroup: "rbac.authorization.k8s.io",
      kind: "ClusterRole",
      name,
    },
    subjects: [
      {
        kind: "ServiceAccount",
        name,
        namespace: POMERIUM_NAMESPACE,
      },
    ],
  };
}
