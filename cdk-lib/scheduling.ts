import { Node, NodeLabelQuery, NodeTaintQuery, TaintEffect, type Workload } from "cdk8s-plus-32";

export interface SchedulingProfile {
  readonly tolerations?: Toleration[];
  readonly affinity?: Affinity;
}

export interface SchedulingTarget {
  readonly queries: NodeLabelQuery[];
  readonly matchExpressions: NodeSelectorRequirement[];
  readonly tolerateControlPlaneWhenRequired?: boolean;
  readonly tolerateControlPlaneWhenPreferred?: boolean;
}

export interface SchedulingConstraint {
  readonly mode: "only" | "prefer";
  readonly target: SchedulingTarget;
  readonly weight: number;
}

export interface Affinity {
  readonly nodeAffinity?: NodeAffinity;
}

export interface NodeAffinity {
  readonly requiredDuringSchedulingIgnoredDuringExecution?: {
    readonly nodeSelectorTerms: NodeSelectorTerm[];
  };
  readonly preferredDuringSchedulingIgnoredDuringExecution?: PreferredSchedulingTerm[];
}

export interface NodeSelectorTerm {
  readonly matchExpressions: NodeSelectorRequirement[];
}

export interface NodeSelectorRequirement {
  readonly key: string;
  readonly operator: string;
  readonly values?: string[];
}

export interface PreferredSchedulingTerm {
  readonly weight: number;
  readonly preference: NodeSelectorTerm;
}

export interface Toleration {
  readonly key: string;
  readonly operator: string;
  readonly effect?: string;
}

const CONTROL_PLANE_LABEL = "node-role.kubernetes.io/control-plane";
const CONTROL_PLANE_TOLERATIONS = [
  { key: CONTROL_PLANE_LABEL, operator: "Exists", effect: "NoSchedule" },
  { key: "CriticalAddonsOnly", operator: "Exists" },
] satisfies Toleration[];

export function workers(): SchedulingTarget {
  return {
    queries: [NodeLabelQuery.doesNotExist(CONTROL_PLANE_LABEL)],
    matchExpressions: [{ key: CONTROL_PLANE_LABEL, operator: "DoesNotExist" }],
    tolerateControlPlaneWhenPreferred: true,
  };
}

export function controlPlane(): SchedulingTarget {
  return {
    queries: [NodeLabelQuery.exists(CONTROL_PLANE_LABEL)],
    matchExpressions: [{ key: CONTROL_PLANE_LABEL, operator: "Exists" }],
    tolerateControlPlaneWhenRequired: true,
    tolerateControlPlaneWhenPreferred: true,
  };
}

export function linux(): SchedulingTarget {
  return {
    queries: [NodeLabelQuery.is("kubernetes.io/os", "linux")],
    matchExpressions: [{ key: "kubernetes.io/os", operator: "In", values: ["linux"] }],
  };
}

export function only(target: SchedulingTarget): SchedulingConstraint {
  return { mode: "only", target, weight: 100 };
}

export function prefer(target: SchedulingTarget, weight = 100): SchedulingConstraint {
  return { mode: "prefer", target, weight };
}

export class Scheduling {
  public static of(workload: Workload): WorkloadScheduling {
    return new WorkloadScheduling(workload);
  }

  public static profile(...constraints: SchedulingConstraint[]): SchedulingProfile {
    const required = constraints.filter(c => c.mode === "only");
    const preferred = constraints.filter(c => c.mode === "prefer");
    return {
      tolerations: tolerationsFor(required, preferred),
      affinity: affinityFor(required, preferred),
    };
  }
}

class WorkloadScheduling {
  public constructor(private readonly workload: Workload) {}

  public apply(...constraints: SchedulingConstraint[]): void {
    const required = constraints.filter(c => c.mode === "only");
    const preferred = constraints.filter(c => c.mode === "prefer");
    this.applyRequired(required);
    this.applyPreferred(preferred);
    this.applyTolerations(required, preferred);
  }

  private applyRequired(constraints: SchedulingConstraint[]): void {
    const queries = constraints.flatMap(c => c.target.queries);
    if (queries.length === 0) return;
    this.workload.scheduling.attract(Node.labeled(...queries));
  }

  private applyPreferred(constraints: SchedulingConstraint[]): void {
    for (const constraint of constraints) {
      this.workload.scheduling.attract(Node.labeled(...constraint.target.queries), { weight: constraint.weight });
    }
  }

  private applyTolerations(required: SchedulingConstraint[], preferred: SchedulingConstraint[]): void {
    if (!shouldTolerateControlPlane(required, preferred)) return;
    this.workload.scheduling.tolerate(
      Node.tainted(NodeTaintQuery.exists(CONTROL_PLANE_LABEL, { effect: TaintEffect.NO_SCHEDULE })),
    );
    this.workload.scheduling.tolerate(Node.tainted(NodeTaintQuery.exists("CriticalAddonsOnly")));
  }
}

function tolerationsFor(required: SchedulingConstraint[], preferred: SchedulingConstraint[]): Toleration[] | undefined {
  if (!shouldTolerateControlPlane(required, preferred)) return undefined;
  return CONTROL_PLANE_TOLERATIONS;
}

function shouldTolerateControlPlane(required: SchedulingConstraint[], preferred: SchedulingConstraint[]): boolean {
  return (
    required.some(c => c.target.tolerateControlPlaneWhenRequired) ||
    preferred.some(c => c.target.tolerateControlPlaneWhenPreferred)
  );
}

function affinityFor(required: SchedulingConstraint[], preferred: SchedulingConstraint[]): Affinity | undefined {
  const nodeAffinity = nodeAffinityFor(required, preferred);
  if (nodeAffinity === undefined) return undefined;
  return { nodeAffinity };
}

function nodeAffinityFor(
  required: SchedulingConstraint[],
  preferred: SchedulingConstraint[],
): NodeAffinity | undefined {
  const requiredExpressions = required.flatMap(c => c.target.matchExpressions);
  const preferredTerms = preferred.map(c => ({
    weight: c.weight,
    preference: { matchExpressions: c.target.matchExpressions },
  }));
  if (requiredExpressions.length === 0 && preferredTerms.length === 0) return undefined;
  return {
    requiredDuringSchedulingIgnoredDuringExecution:
      requiredExpressions.length === 0 ? undefined : { nodeSelectorTerms: [{ matchExpressions: requiredExpressions }] },
    preferredDuringSchedulingIgnoredDuringExecution: preferredTerms.length === 0 ? undefined : preferredTerms,
  };
}
