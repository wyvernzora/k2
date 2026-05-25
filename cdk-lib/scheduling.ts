import type { k8s } from "cdk8s-plus-32";

export interface SchedulingProfile {
  readonly tolerations?: k8s.Toleration[];
  readonly affinity?: k8s.Affinity;
}

const CONTROL_PLANE_MATCH = {
  matchExpressions: [
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "Exists",
    },
  ],
};

const WORKER_MATCH = {
  matchExpressions: [
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "DoesNotExist",
    },
  ],
};

export const Scheduling = {
  controlPlane(): SchedulingProfile {
    return {
      tolerations: [
        { key: "node-role.kubernetes.io/control-plane", operator: "Exists", effect: "NoSchedule" },
        { key: "CriticalAddonsOnly", operator: "Exists" },
      ],
      affinity: {
        nodeAffinity: {
          requiredDuringSchedulingIgnoredDuringExecution: {
            nodeSelectorTerms: [CONTROL_PLANE_MATCH],
          },
        },
      },
    };
  },

  workers(): SchedulingProfile {
    return {
      affinity: {
        nodeAffinity: {
          requiredDuringSchedulingIgnoredDuringExecution: {
            nodeSelectorTerms: [WORKER_MATCH],
          },
        },
      },
    };
  },

  workersPreferred(): SchedulingProfile {
    return {
      tolerations: [
        { key: "node-role.kubernetes.io/control-plane", operator: "Exists", effect: "NoSchedule" },
        { key: "CriticalAddonsOnly", operator: "Exists" },
      ],
      affinity: {
        nodeAffinity: {
          preferredDuringSchedulingIgnoredDuringExecution: [
            {
              weight: 100,
              preference: WORKER_MATCH,
            },
          ],
        },
      },
    };
  },

  controlPlanePreferred(): SchedulingProfile {
    return {
      tolerations: [
        { key: "node-role.kubernetes.io/control-plane", operator: "Exists", effect: "NoSchedule" },
        { key: "CriticalAddonsOnly", operator: "Exists" },
      ],
      affinity: {
        nodeAffinity: {
          preferredDuringSchedulingIgnoredDuringExecution: [
            {
              weight: 100,
              preference: CONTROL_PLANE_MATCH,
            },
          ],
        },
      },
    };
  },
};
