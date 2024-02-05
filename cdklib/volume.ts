import {
  EmptyDirMedium,
  PersistentVolumeAccessMode,
  PersistentVolumeClaim,
  Volume,
  VolumeMount,
} from "cdk8s-plus-27";
import { Construct } from "constructs";
import { Size } from "cdk8s";

const NAS_IP: string = "10.10.8.1";

export type K2VolumeProps =
  | ({ kind: "nas" } & NasVolumeProps)
  | ({ kind: "replicated" } & ReplicatedVolumeProps)
  | ({ kind: "ephemeral" | "memory" } & EphemeralVolumeProps);

export interface NasVolumeProps {
  readonly path: string;
  readonly readOnly?: boolean;
}

export interface ReplicatedVolumeProps {
  readonly size: Size;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

export interface EphemeralVolumeProps {
  readonly sizeLimit?: Size;
}

export class K2Volume extends Construct {
  readonly volume: Volume;

  private constructor(
    scope: Construct,
    id: string,
    init: (scope: K2Volume) => Volume,
  ) {
    super(scope, id);
    this.volume = init(this);
  }

  public mount(props: Omit<VolumeMount, "volume">): VolumeMount {
    return {
      volume: this.volume,
      ...props,
    };
  }

  public static fromProps(
    scope: Construct,
    id: string,
    props: K2VolumeProps,
  ): K2Volume {
    switch (props.kind) {
      case "nas":
        return K2Volume.nas(scope, id, props);
      case "replicated":
        return K2Volume.replicated(scope, id, props);
      case "ephemeral":
        return K2Volume.ephemeral(scope, id, props);
      case "memory":
        return K2Volume.memory(scope, id, props);
    }
  }

  public static nas(
    scope: Construct,
    id: string,
    props: NasVolumeProps,
  ): K2Volume {
    return new K2Volume(scope, id, (s) => {
      return Volume.fromNfs(s, "vol", `rumi-${s.node.id}`, {
        server: NAS_IP,
        path: props.path,
        readOnly: props.readOnly,
      });
    });
  }

  public static replicated(
    scope: Construct,
    id: string,
    props: ReplicatedVolumeProps,
  ): K2Volume {
    return new K2Volume(scope, id, (s) => {
      const pvc = new PersistentVolumeClaim(s, `pvc`, {
        storageClassName: "longhorn",
        accessModes: props.accessModes || [
          PersistentVolumeAccessMode.READ_WRITE_ONCE,
        ],
        storage: props.size,
      });
      return Volume.fromPersistentVolumeClaim(s, `vol`, pvc);
    });
  }

  public static ephemeral(
    scope: Construct,
    id: string,
    props: EphemeralVolumeProps = {},
  ): K2Volume {
    return new K2Volume(scope, id, (s) =>
      Volume.fromEmptyDir(s, `vol`, `eph-${s.node.id}`, {
        sizeLimit: props.sizeLimit,
      }),
    );
  }

  public static memory(
    scope: Construct,
    id: string,
    props: EphemeralVolumeProps = {},
  ): K2Volume {
    return new K2Volume(scope, id, (s) =>
      Volume.fromEmptyDir(s, `vol`, `mem-${s.node.id}`, {
        medium: EmptyDirMedium.MEMORY,
        sizeLimit: props.sizeLimit,
      }),
    );
  }
}
