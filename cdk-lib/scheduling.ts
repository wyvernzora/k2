import { Node, NodeLabelQuery, NodeTaintQuery, TaintEffect, type Workload } from "cdk8s-plus-32";
import type { k8s } from "cdk8s-plus-32";

export interface SchedulingProfile {
  readonly tolerations?: k8s.Toleration[];
  readonly affinity?: k8s.Affinity;
}

const CONTROL_PLANE_LABEL = "node-role.kubernetes.io/control-plane";

const CONTROL_PLANE_MATCH = {
  matchExpressions: [
    {
      key: CONTROL_PLANE_LABEL,
      operator: "Exists",
    },
  ],
};

const WORKER_MATCH = {
  matchExpressions: [
    {
      key: CONTROL_PLANE_LABEL,
      operator: "DoesNotExist",
    },
  ],
};

export const Scheduling = {
  tolerateControlPlane(workload: Workload): void {
    workload.scheduling.tolerate(
      Node.tainted(NodeTaintQuery.exists(CONTROL_PLANE_LABEL, { effect: TaintEffect.NO_SCHEDULE })),
    );
    workload.scheduling.tolerate(Node.tainted(NodeTaintQuery.exists("CriticalAddonsOnly")));
  },

  applyWorkersPreferred(workload: Workload): void {
    Scheduling.tolerateControlPlane(workload);
    workload.scheduling.attract(Node.labeled(NodeLabelQuery.doesNotExist(CONTROL_PLANE_LABEL)), { weight: 100 });
  },

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
