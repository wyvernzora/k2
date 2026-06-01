import { Size } from "cdk8s";
import { PersistentVolumeAccessMode, Volume, type VolumeMount, type Workload } from "cdk8s-plus-32";
import type { Construct } from "constructs";

export interface MaterializedVolume {
  readonly volume: Volume;
  configureWorkload(workload: Workload): void;
}

export type K2Volumes = Record<string, K2Volume>;

export type K2Mounters<V extends K2Volumes> = {
  [K in keyof V]: (path: string, opts?: K2MountOptions) => VolumeMount;
};

export type K2MountOptions = Omit<VolumeMount, "volume" | "path">;

export interface K2EphemeralProps {
  readonly sizeLimit?: Size;
}

export interface K2NfsProps {
  readonly path: string;
  readonly readOnly?: boolean;
}

/**
 * Dynamically provision a new NFS-backed PVC through the cluster's NFS CSI
 * StorageClass. This is for new Kubernetes-owned directories, not importing an
 * existing NFS export path; use {@link K2Volume.mountNfs} for existing paths.
 */
export interface K2ProvisionedNfsProps {
  readonly name?: string;
  readonly size: Size;
  readonly storageClass?: string;
  readonly accessModes?: PersistentVolumeAccessMode[];
  readonly readOnly?: boolean;
}

export interface K2ReplicatedProps {
  readonly name?: string;
  readonly size: Size;
  readonly storageClass?: string;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

export abstract class K2Volume {
  public abstract materialize(scope: Construct, id: string): MaterializedVolume;

  /**
   * Static factories below are initialized in `volumes/index.ts` to break the
   * import cycle that would otherwise exist between this file and the concrete
   * subclasses in `ephemeral.ts`, `nfs.ts`, `replicated.ts` (all of which
   * `extends K2Volume`). Anyone importing `K2Volume` via `@k2/cdk-lib` or
   * `cdk-lib/volumes/index.js` gets the factories assigned by the time they
   * call them.
   */
  declare public static ephemeral: (props?: K2EphemeralProps) => K2Volume;
  declare public static mountNfs: (props: K2NfsProps) => K2Volume;
  declare public static provisionNfs: (props: K2ProvisionedNfsProps) => K2Volume;
  declare public static replicated: (props: K2ReplicatedProps) => K2Volume;
}

export class SimpleMaterializedVolume implements MaterializedVolume {
  public constructor(public readonly volume: Volume) {}

  public configureWorkload(): void {
    // cdk8s-plus adds the volume to the workload automatically when a
    // VolumeMount references it.
  }
}
