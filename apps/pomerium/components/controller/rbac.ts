import { ApiResource, ClusterRole, ClusterRoleBinding, type IServiceAccount } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { POMERIUM_CONTROLLER_NAME } from "../../constants.js";

import { clusterMetadata } from "./metadata.js";

export const GEN_SECRETS_SERVICE_ACCOUNT = "pomerium-gen-secrets";

export function createRbac(
  scope: Construct,
  controllerServiceAccount: IServiceAccount,
  genSecretsServiceAccount: IServiceAccount,
): void {
  const controllerRole = new ClusterRole(scope, "controller-cluster-role", controllerClusterRole());
  const genSecretsRole = new ClusterRole(scope, "gen-secrets-cluster-role", genSecretsClusterRole());

  new ClusterRoleBinding(scope, "controller-cluster-role-binding", {
    metadata: clusterMetadata(POMERIUM_CONTROLLER_NAME),
    role: controllerRole,
  }).addSubjects(controllerServiceAccount);
  new ClusterRoleBinding(scope, "gen-secrets-cluster-role-binding", {
    metadata: clusterMetadata(GEN_SECRETS_SERVICE_ACCOUNT),
    role: genSecretsRole,
  }).addSubjects(genSecretsServiceAccount);
}

function controllerClusterRole() {
  return {
    metadata: clusterMetadata(POMERIUM_CONTROLLER_NAME),
    rules: [
      rbacRule(["get", "list", "watch"], ApiResource.SERVICES, ApiResource.ENDPOINTS, ApiResource.SECRETS),
      rbacRule(
        ["get"],
        coreResource("services/status"),
        coreResource("secrets/status"),
        coreResource("endpoints/status"),
      ),
      rbacRule(["get", "list", "watch"], ApiResource.INGRESSES, ApiResource.INGRESS_CLASSES),
      rbacRule(["get", "patch", "update"], networkingResource("ingresses/status")),
      rbacRule(["get", "list", "watch"], pomeriumResource("pomerium")),
      rbacRule(["get", "update", "patch"], pomeriumResource("pomerium/status")),
      rbacRule(["create", "patch"], ApiResource.EVENTS),
    ],
  };
}

function genSecretsClusterRole() {
  return {
    metadata: clusterMetadata(GEN_SECRETS_SERVICE_ACCOUNT),
    rules: [rbacRule(["create", "get"], ApiResource.SECRETS)],
  };
}

function rbacRule(verbs: string[], ...resources: ApiResource[]) {
  return { verbs, endpoints: resources };
}

function coreResource(resourceType: string): ApiResource {
  return ApiResource.custom({ apiGroup: "", resourceType });
}

function networkingResource(resourceType: string): ApiResource {
  return ApiResource.custom({ apiGroup: "networking.k8s.io", resourceType });
}

function pomeriumResource(resourceType: string): ApiResource {
  return ApiResource.custom({ apiGroup: "ingress.pomerium.io", resourceType });
}
