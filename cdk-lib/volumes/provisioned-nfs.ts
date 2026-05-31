import { PersistentVolumeAccessMode, PersistentVolumeClaim, Volume, type Workload } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { NfsContext } from "../context/nfs.js";

import { K2Volume, SimpleMaterializedVolume, type K2ProvisionedNfsProps, type MaterializedVolume } from "./base.js";
import { configureNfsWorkloadAffinity } from "./nfs-affinity.js";

const DEFAULT_NFS_CSI_STORAGE_CLASS = "nfs-csi";

export class K2ProvisionedNfsVolume extends K2Volume {
  public constructor(private readonly props: K2ProvisionedNfsProps) {
    super();
  }

  public materialize(scope: Construct, id: string): MaterializedVolume {
    const zone = NfsContext.of(scope).zone;
    const claim = new PersistentVolumeClaim(scope, `${id}-claim`, {
      storage: this.props.size,
      storageClassName: this.props.storageClass ?? DEFAULT_NFS_CSI_STORAGE_CLASS,
      accessModes: this.props.accessModes ?? [PersistentVolumeAccessMode.READ_WRITE_MANY],
    });
    const volume = Volume.fromPersistentVolumeClaim(scope, id, claim, {
      readOnly: this.props.readOnly,
    });
    return {
      ...new SimpleMaterializedVolume(volume),
      configureWorkload(workload: Workload): void {
        configureNfsWorkloadAffinity(workload, zone);
      },
    };
  }
}
