import { PersistentVolumeAccessMode, PersistentVolumeClaim, Volume } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Volume, SimpleMaterializedVolume, type K2ReplicatedProps, type MaterializedVolume } from "./base.js";

export class K2ReplicatedVolume extends K2Volume {
  public constructor(private readonly props: K2ReplicatedProps) {
    super();
  }

  public materialize(scope: Construct, id: string): MaterializedVolume {
    const claim = new PersistentVolumeClaim(scope, `${id}-claim`, {
      storage: this.props.size,
      storageClassName: this.props.storageClass ?? "longhorn",
      accessModes: this.props.accessModes ?? [PersistentVolumeAccessMode.READ_WRITE_ONCE],
    });
    const volume = Volume.fromPersistentVolumeClaim(scope, id, claim);
    return new SimpleMaterializedVolume(volume);
  }
}
