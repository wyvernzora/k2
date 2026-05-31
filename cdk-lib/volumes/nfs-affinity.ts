import { LabeledNode, NodeLabelQuery, type Workload } from "cdk8s-plus-32";

const configuredWorkloads = new WeakMap<Workload, string>();

export function configureNfsWorkloadAffinity(workload: Workload, zone?: string): void {
  if (zone === undefined || configuredWorkloads.get(workload) === zone) {
    return;
  }
  if (configuredWorkloads.has(workload)) {
    throw new Error("Cannot apply multiple NFS zone preferences to the same workload");
  }
  configuredWorkloads.set(workload, zone);
  workload.scheduling.attract(new LabeledNode([NodeLabelQuery.is("topology.kubernetes.io/zone", zone)]), {
    weight: 100,
  });
}
