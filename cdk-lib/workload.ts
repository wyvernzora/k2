import { Deployment, type DeploymentProps } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { only, Scheduling, workers } from "./scheduling.js";
import type { K2Mounters, K2Volumes } from "./volumes/base.js";

export class K2Deployment extends Deployment {
  /**
   * Claim name per attached volume, for the few apps that mount the same claim
   * from a second object -- a Job, say. Reading it here rather than naming the
   * claim in both places keeps the two from drifting apart, which matters most
   * during a migration: the workload moves to the destination claim and
   * anything still naming the source is left on a volume about to be pruned.
   * Absent for volume types that emit no PVC, such as an NFS mount.
   */
  public readonly volumeClaims: Record<string, string | undefined> = {};

  public constructor(scope: Construct, id: string, props: DeploymentProps = {}) {
    super(scope, id, props);
    Scheduling.of(this).apply(only(workers()));
  }

  public attachVolumes<V extends K2Volumes>(volumes: V): K2Mounters<V> {
    const out = {} as K2Mounters<V>;
    for (const [name, volume] of Object.entries(volumes)) {
      const materialized = volume.materialize(this, `vol-${name}`);
      materialized.configureWorkload(this);
      this.volumeClaims[name] = materialized.claimName;
      out[name as keyof V] = (path, opts) => ({
        volume: materialized.volume,
        path,
        ...opts,
      });
    }
    return out;
  }
}
