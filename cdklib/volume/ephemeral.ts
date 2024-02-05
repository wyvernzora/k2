import { Size } from "cdk8s";
import { IMountable, MountProps } from "~lib";
import { Construct } from "constructs";
import { EmptyDirMedium, Volume, VolumeMount } from "cdk8s-plus-27";

export interface EphemeralVolumeProps {
  readonly kind: "ephemeral";
  readonly sizeLimit?: Size;
  readonly medium?: EmptyDirMedium;
}

export function createEphemeralVolume(
  scope: Construct,
  id: string,
  props: EphemeralVolumeProps,
): IMountable {
  return new (class extends Construct implements IMountable {
    private readonly volume: Volume;

    constructor() {
      super(scope, id);
      this.volume = Volume.fromEmptyDir(this, `vol`, `eph-${this.node.id}`, {
        sizeLimit: props.sizeLimit,
        medium: props.medium,
      });
    }

    public mount(props: MountProps): VolumeMount {
      return {
        volume: this.volume,
        ...props,
      };
    }
  })();
}
