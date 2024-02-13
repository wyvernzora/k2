import {
  EmptyDirMedium,
  IStorage,
  LabeledNode,
  NodeLabelQuery,
  PersistentVolumeAccessMode,
  PersistentVolumeClaim,
  Volume,
  VolumeMount,
  Workload,
} from "cdk8s-plus-27";
import { Construct, IConstruct } from "constructs";
import { Size } from "cdk8s";

/**
 * K2Volume represents a piece of mountable storage within the K2 cluster.
 * While it creates vanilla Kubernetes volumes under the hood, there are multiple
 * implementations that handle some very K2-specific bits and pieces.
 */
export type K2Volume = (scope: IConstruct, id: string) => K2MaterializedVolume;
export namespace K2Volume {
  /**
   * Creates a constructor for an ephemeral volume that goes away when the pod
   * it's attached to is deleted.
   */
  export function ephemeral(props?: K2EphemeralVolumeProps): K2Volume {
    return K2EphemeralVolume.of({ ...props });
  }

  /**
   * Creates a constructor for a replicated volume that is fault-tolerant and is
   * always available on any given node.
   */
  export function replicated(props: K2ReplicatedVolumeProps): K2Volume {
    return K2ReplicatedVolume.of(props);
  }

  /**
   * Creates a constructor for a bulk storage volume with high capacity, but not
   * necessarily availability across zones. This type of volume configures its
   * workload to prefer running in the same zone as the bulk storage server.
   */
  export function bulk(props: K2BulkVolumeProps): K2Volume {
    return K2BulkVolume.of(props);
  }
}

/**
 * A map of named volumes, to be used in props.
 */
export type K2Volumes<Name extends string = string> = Record<Name, K2Volume>;

/**
 * K2MaterializedVolume is a K2Volume that has been turned into an actual Volume
 * construct within the constructs tree.
 */
export abstract class K2MaterializedVolume
  extends Construct
  implements IStorage
{
  public abstract asVolume(): Volume;

  /**
   * Configures the workload that this volume is attached to, so that it can show
   * any behavior that is desired by the implementing node type.
   */
  protected abstract configure(workload: Workload): void;

  /**
   * Creates a VolumeMount object to be attached to specific containers.
   */
  public mount(workload: Workload, props: K2MountOptions): VolumeMount {
    this.configure(workload);
    return {
      volume: this.asVolume(),
      ...props,
    };
  }
}

/**
 * Props for creating VolumeMount object using K2Volume.
 */
export type K2MountOptions = Omit<VolumeMount, "volume">;

export interface K2EphemeralVolumeProps {
  readonly sizeLimit?: Size;
  readonly medium?: EmptyDirMedium;
}

/**
 * K2EphemeralVolume represents a storage that is cleared when a pod is deleted.
 * Semantically the same as empty dir volume, but implementation may vary.
 */
export class K2EphemeralVolume extends K2MaterializedVolume {
  private readonly volume: Volume;

  static of(props: K2EphemeralVolumeProps): K2Volume {
    return (scope, id) => new K2EphemeralVolume(scope, id, props);
  }

  constructor(scope: IConstruct, id: string, props: K2EphemeralVolumeProps) {
    super(scope, id);
    this.volume = Volume.fromEmptyDir(this, `vol`, `eph-${this.node.id}`, {
      sizeLimit: props.sizeLimit,
      medium: props.medium,
    });
  }

  public asVolume(): Volume {
    return this.volume;
  }

  protected configure() {}
}

export interface K2ReplicatedVolumeProps {
  readonly size: Size;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

/**
 * K2ReplicatedVolume is storage that is replicated across multiple nodes in the cluster
 * and is fault-tolerant to losing a single cluster zone.
 */
export class K2ReplicatedVolume extends K2MaterializedVolume {
  public static readonly STORAGE_CLASS = "longhorn";

  public static of(props: K2ReplicatedVolumeProps): K2Volume {
    return (scope, id) => new K2ReplicatedVolume(scope, id, props);
  }

  private readonly pvc: PersistentVolumeClaim;
  private readonly volume: Volume;

  constructor(scope: Construct, id: string, props: K2ReplicatedVolumeProps) {
    super(scope, id);
    this.pvc = new PersistentVolumeClaim(this, `pvc`, {
      storageClassName: K2ReplicatedVolume.STORAGE_CLASS,
      accessModes: props.accessModes || [
        PersistentVolumeAccessMode.READ_WRITE_ONCE,
      ],
      storage: props.size,
    });
    this.volume = Volume.fromPersistentVolumeClaim(this, `vol`, this.pvc);
  }

  public asVolume(): Volume {
    return this.volume;
  }

  protected configure() {}
}

export interface K2BulkVolumeProps {
  readonly path: string;
  readonly readOnly?: boolean;
}

/**
 * K2NasVolume is storage on a centralized storage server that has large capacity
 * but is not replicated across zones.
 */
export class K2BulkVolume extends K2MaterializedVolume {
  public static readonly NAS_IP = "10.10.8.1";
  public static readonly NAS_ZONE = "roxy";

  public static of(props: K2BulkVolumeProps): K2Volume {
    return (scope, id) => new K2BulkVolume(scope, id, props);
  }

  private readonly volume: Volume;

  constructor(scope: IConstruct, id: string, props: K2BulkVolumeProps) {
    super(scope, id);
    this.volume = Volume.fromNfs(this, "vol", `blk-${this.node.id}`, {
      server: K2BulkVolume.NAS_IP,
      path: props.path,
      readOnly: props.readOnly,
    });
  }

  public asVolume(): Volume {
    return this.volume;
  }

  protected configure(workload: Workload): void {
    // Prefer scheduling in the same zone as storage server
    const labeledNode = new LabeledNode([
      NodeLabelQuery.is("topology.kubernetes.io/zone", K2BulkVolume.NAS_ZONE),
    ]);
    workload.scheduling.attract(labeledNode);
  }
}
