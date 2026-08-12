import { PersistentVolumeAccessMode, PersistentVolumeClaim, Volume } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  K2Volume,
  SimpleMaterializedVolume,
  volumeIdSuffix,
  type K2IscsiProps,
  type MaterializedVolume,
} from "./base.js";

/**
 * A PVC on the storage appliance's iSCSI class.
 *
 * ReadWriteOnce only, and deliberately not defaulted otherwise: the appliance
 * exports block devices, so a second concurrent writer corrupts the
 * filesystem rather than merely conflicting. Callers that need shared access
 * want NFS, not this.
 *
 * The claim name is derived from the construct id and the volume type rather
 * than left to cdk8s' path-based default, so that this volume keeps the same
 * name whether it is declared directly or as a `migrate` destination. That is
 * what lets the migrate wrapper be removed with no rename and no operator
 * step. See {@link migratableClaimName}.
 */
export class K2IscsiVolume extends K2Volume {
  public constructor(private readonly props: K2IscsiProps) {
    super();
  }

  public materialize(scope: Construct, id: string, identity?: string): MaterializedVolume {
    const storageClassName = this.props.storageClass ?? "k2-iscsi";
    // Built under `<identity>-<suffix>` so this volume lands in the same place
    // whether it is declared directly or as a migrate destination, and never on
    // top of the claim it is migrating from.
    const base = `${identity ?? id}-${volumeIdSuffix(storageClassName)}`;
    const claim = new PersistentVolumeClaim(scope, `${base}-claim`, {
      metadata: this.props.name === undefined ? undefined : { name: this.props.name },
      storage: this.props.size,
      storageClassName,
      accessModes: this.props.accessModes ?? [PersistentVolumeAccessMode.READ_WRITE_ONCE],
    });
    const volume = Volume.fromPersistentVolumeClaim(scope, base, claim);
    return new SimpleMaterializedVolume(volume, claim.name);
  }
}
