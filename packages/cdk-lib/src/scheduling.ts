import { k8s } from "cdk8s-plus-28";

/**
 * Common tolerations useful when synthesizing Helm charts
 */
export const Toleration = {
  /**
   * Allows scheduling of the pod on nodes marked for critical addons only.
   */
  ALLOW_CRITICAL_ADDONS_ONLY: [
    {
      key: "CriticalAddonsOnly",
      operator: "Exists",
    },
  ],
  /**
   * Allows scheduling of the pod on control plane nodes.
   */
  ALLOW_CONTROL_PLANE: [
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "Exists",
      effect: "NoSchedule",
    },
    {
      key: "node-role.kubernetes.io/master",
      operator: "Exists",
      effect: "NoSchedule",
    },
  ],
};

function annotatedNode(annotation: string): k8s.NodeSelectorTerm {
  return {
    matchExpressions: [
      {
        key: annotation,
        operator: "Exists",
      },
    ],
  };
}

function preferAnyOf(...terms: k8s.NodeSelectorTerm[]): k8s.NodeAffinity {
  return {
    preferredDuringSchedulingIgnoredDuringExecution: terms.map((term) => ({
      weight: 1,
      preference: term,
    })),
  };
}

function requireAnyOf(...terms: k8s.NodeSelectorTerm[]): k8s.NodeAffinity {
  return {
    requiredDuringSchedulingIgnoredDuringExecution: {
      nodeSelectorTerms: terms,
    },
  };
}

/**
 * Common affinities useful when synthesizing Helm charts.
 */
export const NodeAffinity = {
  /**
   * Prefers the pod to be scheduled on the control plane node
   */
  PREFER_CONTROL_PLANE: preferAnyOf(
    annotatedNode("node-role.kubernetes.io/control-plane"),
    annotatedNode("node-role.kubernetes.io/master"),
  ),
  /**
   * Requires the pod to be scheduled on the control plane node
   */
  REQUIRE_CONTROL_PLANE: requireAnyOf(
    annotatedNode("node-role.kubernetes.io/control-plane"),
    annotatedNode("node-role.kubernetes.io/master"),
  ),
};
