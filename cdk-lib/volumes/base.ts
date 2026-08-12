import { createHash } from "node:crypto";

import { Size } from "cdk8s";
import { PersistentVolumeAccessMode, Volume, type VolumeMount, type Workload } from "cdk8s-plus-32";
import type { Construct } from "constructs";

export interface MaterializedVolume {
  readonly volume: Volume;
  /** Set by types that emit a PVC; used to detect a migration whose two claims would collide. */
  readonly claimName?: string;
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

export interface K2IscsiProps {
  readonly name?: string;
  readonly size: Size;
  readonly storageClass?: string;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

/**
 * Copy `from` into `to` once, then run the workload on `to`. The source is
 * mounted read-only and never deleted — reclaiming it is a deliberate step
 * after the destination has soaked.
 */
export interface K2MigrateProps {
  readonly from: K2Volume;
  readonly to: K2Volume;
  readonly image?: string;
  readonly initContainerName?: string;
  readonly markerFile?: string;
}

export interface K2ReplicatedProps {
  readonly name?: string;
  readonly size: Size;
  readonly storageClass?: string;
  readonly accessModes?: PersistentVolumeAccessMode[];
}

/**
 * Opaque construct-id suffix distinguishing one volume's claim from another's
 * at the same declared id.
 *
 * cdk8s derives resource names from the construct path alone, so two volume
 * types built at the same id produce the same claim name. Rather than compute
 * names by hand, the volume type builds its claim under `<id>-<suffix>`, and
 * cdk8s names it from there as usual. That keeps naming — length limits, DNS
 * rules, hashing — entirely cdk8s' business.
 *
 * The suffix is derived from the STORAGE CLASS, not the size: a class cannot be
 * changed on an existing PVC, so a different class genuinely needs a different
 * volume, while a size change can be applied in place. Deriving it from size
 * would silently rename — and so empty — a volume being expanded.
 *
 * Four hex characters, deliberately opaque: it exists to disambiguate, and
 * nothing should come to depend on reading it.
 *
 * This only affects claims whose name cdk8s generates. An explicit `name` is
 * used verbatim, so a volume declared with one is unaffected by any of this —
 * which is how every claim in this repo except one is declared.
 */
export function volumeIdSuffix(discriminator: string): string {
  return createHash("sha256").update(discriminator).digest("hex").slice(0, 4);
}

export abstract class K2Volume {
  /**
   * `identity` overrides the id a volume builds its constructs under, so a
   * volume materialized somewhere else is built exactly as if it had been
   * declared at `identity`. Migration destinations pass it — see
   * {@link volumeIdSuffix}. Types that emit no PVC ignore it.
   */
  public abstract materialize(scope: Construct, id: string, identity?: string): MaterializedVolume;

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
  declare public static iscsi: (props: K2IscsiProps) => K2Volume;
  declare public static migrate: (props: K2MigrateProps) => K2Volume;
}

export class SimpleMaterializedVolume implements MaterializedVolume {
  public constructor(
    public readonly volume: Volume,
    public readonly claimName?: string,
  ) {}

  public configureWorkload(): void {
    // cdk8s-plus adds the volume to the workload automatically when a
    // VolumeMount references it.
  }
}
