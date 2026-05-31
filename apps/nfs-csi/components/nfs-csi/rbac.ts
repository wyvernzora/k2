import { ApiObject, JsonPatch, type Helm } from "cdk8s";
import { ApiResource, Role, ServiceAccount } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { Namespace } from "@k2/cdk-lib";

const NFS_PROVISIONER_CLUSTER_ROLE = "nfs-external-provisioner-role";
const NFS_CONTROLLER_SERVICE_ACCOUNT = "csi-nfs-controller-sa";

export function tightenRbac(scope: Construct, chart: Helm): void {
  addNamespacedLeaseRole(scope);
  stripClusterRoleRules(chart, isUnneededClusterRoleRule);
}

function addNamespacedLeaseRole(scope: Construct): void {
  const serviceAccount = ServiceAccount.fromServiceAccountName(
    scope,
    "controller-service-account-ref",
    NFS_CONTROLLER_SERVICE_ACCOUNT,
    { namespaceName: Namespace.of(scope).namespace },
  );
  const role = new Role(scope, "controller-leader-election-role", {
    metadata: { name: `${NFS_CONTROLLER_SERVICE_ACCOUNT}-leader-election` },
    rules: [
      {
        verbs: ["get", "list", "watch", "create", "update", "patch"],
        resources: [ApiResource.LEASES],
      },
    ],
  });
  role.bind(serviceAccount);
}

function stripClusterRoleRules(chart: Helm, shouldRemove: (rule: RbacRule) => boolean): void {
  let matchedClusterRoles = 0;
  for (const resource of chart.apiObjects) {
    if (resource.kind === "ClusterRole") {
      stripRbacRules(resource, shouldRemove);
      matchedClusterRoles++;
    }
  }
  if (matchedClusterRoles === 0) {
    throw new Error("Expected ClusterRole resources in csi-driver-nfs chart output");
  }
}

function stripRbacRules(resource: ApiObject, shouldRemove: (rule: RbacRule) => boolean): void {
  const rules = ((resource.toJson() as { rules?: RbacRule[] }).rules ?? []).map((rule, index) => ({ rule, index }));
  const removals = rules
    .filter(({ rule }) => shouldRemove(rule))
    .map(({ index }) => index)
    .sort((left, right) => right - left);
  if (resource.metadata.name === NFS_PROVISIONER_CLUSTER_ROLE && removals.length !== 5) {
    throw new Error(`Expected five unneeded ${NFS_PROVISIONER_CLUSTER_ROLE} RBAC rules, found ${removals.length}`);
  }
  resource.addJsonPatch(...removals.map(index => JsonPatch.remove(`/rules/${index}`)));
}

function isUnneededClusterRoleRule(rule: RbacRule): boolean {
  return isSnapshotRule(rule) || isSecretRule(rule) || isLeaseRule(rule);
}

function isSnapshotRule(rule: RbacRule): boolean {
  return rule.apiGroups?.includes("snapshot.storage.k8s.io") === true;
}

function isSecretRule(rule: RbacRule): boolean {
  return rule.apiGroups?.includes("") === true && rule.resources?.includes("secrets") === true;
}

function isLeaseRule(rule: RbacRule): boolean {
  return rule.apiGroups?.includes("coordination.k8s.io") === true && rule.resources?.includes("leases") === true;
}

interface RbacRule {
  readonly apiGroups?: string[];
  readonly resources?: string[];
}
