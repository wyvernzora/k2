import { Size } from "cdk8s";
import {
  PersistentVolumeAccessMode,
  PersistentVolumeClaim,
  Volume,
  VolumeMount,
} from "cdk8s-plus-27";
import { IMountable, MountProps } from "~lib/volume";
import { Construct } from "constructs";

export interface ReplicatedVolumeProps {
  readonly kind: "replicated";
  readonly size: Size;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

export function createReplicatedVolume(
  scope: Construct,
  id: string,
  props: ReplicatedVolumeProps,
): IMountable {
  return new (class extends Construct implements IMountable {
    private readonly volume: Volume;

    constructor() {
      super(scope, id);
      const pvc = new PersistentVolumeClaim(this, `pvc`, {
        storageClassName: "longhorn",
        accessModes: props.accessModes || [
          PersistentVolumeAccessMode.READ_WRITE_ONCE,
        ],
        storage: props.size,
      });
      this.volume = Volume.fromPersistentVolumeClaim(this, `vol`, pvc);
    }

    public mount(props: MountProps): VolumeMount {
      return {
        volume: this.volume,
        ...props,
      };
    }
  })();
}
