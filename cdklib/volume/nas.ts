import { Volume, VolumeMount } from "cdk8s-plus-27";
import { Construct } from "constructs";
import { join } from "path";
import type { IMountable, MountProps } from ".";

export interface NasVolumeProps {
  readonly kind: "nas";
  readonly path: string;
  readonly subPath?: string;
  readonly readOnly?: boolean;
}

export function createNasVolume(
  scope: Construct,
  id: string,
  props: Omit<NasVolumeProps, "kind">,
): IMountable {
  return new (class extends Construct implements IMountable {
    private readonly volume: Volume;
    private readonly subPath?: string;

    constructor() {
      super(scope, id);
      this.subPath = props.subPath;
      this.volume = Volume.fromNfs(this, "vol", `nas-${this.node.id}`, {
        server: "10.10.8.1",
        path: props.path,
        readOnly: props.readOnly,
      });
    }

    public mount(props: MountProps): VolumeMount {
      const subPath = join(
        this.subPath || "",
        props.subPath || props.subPathExpr || "",
      );
      return {
        volume: this.volume,
        ...props,
        subPath,
      };
    }
  })();
}
