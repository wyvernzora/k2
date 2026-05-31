import { Volume, type Workload } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { NfsContext } from "../context/nfs.js";

import { K2Volume, type K2NfsProps, type MaterializedVolume } from "./base.js";
import { configureNfsWorkloadAffinity } from "./nfs-affinity.js";

export class K2NfsVolume extends K2Volume {
  public constructor(private readonly props: K2NfsProps) {
    super();
  }

  /**
   * Materialize the NFS volume and, when {@link NfsContext.zone} is set, apply
   * a *soft* node-affinity to the workload that hosts this volume, attracting
   * the pod toward the NAS-local zone. This side effect runs in
   * {@link MaterializedVolume.configureWorkload}; if a caller materializes the
   * volume but never mounts it on a container, the affinity is still applied.
   */
  public materialize(scope: Construct, id: string): MaterializedVolume {
    const nfs = NfsContext.of(scope);
    const volume = Volume.fromNfs(scope, id, id, {
      server: nfs.server,
      path: this.props.path,
      readOnly: this.props.readOnly,
    });
    const zone = nfs.zone;
    return {
      volume,
      configureWorkload(workload: Workload): void {
        configureNfsWorkloadAffinity(workload, zone);
      },
    };
  }
}
