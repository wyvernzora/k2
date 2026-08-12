import { type ContainerSecurityContextProps, Volume, type Workload } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Volume, type K2MigrateProps, type MaterializedVolume } from "./base.js";

/**
 * A one-shot copy from an existing volume into a new one, used to move an
 * app's data between storage classes without editing the app.
 *
 * The workload mounts the DESTINATION; the source is mounted read-only into
 * an init container that copies it once. Shape and constraints:
 *
 * - Single-writer only. The copy runs as an init container in the app's own
 *   pod, so it is safe exactly when the app itself is not running — i.e. a
 *   single-replica `Recreate` workload. Anything scaled out or RollingUpdate
 *   would copy underneath a live writer.
 * - Idempotent by completion marker, not by rsync semantics. A partial copy
 *   (node reboot, OOM) leaves no marker and is redone from scratch on the
 *   next start; a completed copy is skipped. Without the marker an interrupted
 *   run would look identical to a finished one.
 * - The SOURCE IS NEVER DELETED and never written. Reclaiming it is a
 *   deliberate operator step after the destination has soaked, because the
 *   only cheap undo during a migration is "point back at the source".
 * - `from` still EMITS a PVC, so for a live app it must declare the existing
 *   claim exactly as it stands today — apply adopts a match, but storage
 *   class, size and access modes are immutable, so a PVC that was expanded
 *   past its declared size fails to apply rather than migrating. It must keep
 *   emitting one for the whole migration: stop declaring it and Argo prunes
 *   the claim, which on a Delete-reclaim class destroys the data.
 *
 * The whole lifecycle is three edits and two syncs, with no operator commands
 * and no intermediate state to remember:
 *
 *   1. wrap:    `v`  ->  `migrate({from: v, to: w})`   deploy; the copy runs
 *   2. unwrap:  `migrate({from: v, to: w})`  ->  `w`   deploy; done
 *
 * Both steps are safe because of how the two claims are named:
 *
 * - The SOURCE materializes under the SAME construct id this volume was given,
 *   so `migrate({from: v, ...})` emits byte-identical PVC objects to a bare
 *   `v`. Wrapping a live volume is transparent and adopts the existing claim.
 * - The DESTINATION is named from a SEED — the id this volume was declared
 *   under — rather than from its own construct path, so it is named as though
 *   it had been declared directly. Unwrapping to a bare `w` therefore emits
 *   the identical claim: no rename, no rebind, nothing to migrate twice.
 * - A type discriminator in that name keeps the two claims distinct while they
 *   coexist, which they must, since cdk8s would otherwise derive the same name
 *   for both (it hashes the construct path, ignoring kind and storage class).
 *
 * Unwrapping stops declaring the source, so Argo prunes it — on a
 * Delete-reclaim class that is what actually frees the old volume. Do not
 * unwrap until the destination has soaked and is backed up.
 *
 * Destination support is per volume type: it requires seed-based naming, which
 * `K2Volume.iscsi` has. `K2Volume.replicated` deliberately keeps cdk8s' default
 * naming so the existing Longhorn claims across the cluster are not renamed,
 * so it works as a SOURCE but not yet as a destination.
 */
export class K2MigrateVolume extends K2Volume {
  public constructor(private readonly props: K2MigrateProps) {
    super();
  }

  public materialize(scope: Construct, id: string): MaterializedVolume {
    const source = this.props.from.materialize(scope, id);
    // Seeded with the id this volume was declared under, so the destination
    // claim is named as if it were declared there directly. Removing the
    // wrapper then changes nothing about it.
    let destination: MaterializedVolume;
    try {
      destination = this.props.to.materialize(scope, `${id}-destination`, id);
    } catch (error) {
      // A clash means both volumes derived the same construct id, i.e. the same
      // storage class. cdk8s' own message does not explain why that is fatal.
      if (error instanceof Error && /already a construct|already exists/i.test(error.message)) {
        throw new Error(sameStorageClassMessage(id), { cause: error });
      }
      throw error;
    }
    if (source.claimName !== undefined && source.claimName === destination.claimName) {
      throw new Error(sameStorageClassMessage(id));
    }
    return new MigratingVolume(source, destination, this.props);
  }
}

function sameStorageClassMessage(id: string): string {
  return (
    `migrate(${id}): the source and destination resolve to the same volume. ` +
    "Migrating within a single storage class is not supported: the two claims must coexist while " +
    "the copy runs, and they are told apart by storage class, so there is nothing left to " +
    "distinguish them. Migrate to a different class, or resize in place if the class allows " +
    "expansion."
  );
}

class MigratingVolume implements MaterializedVolume {
  public constructor(
    private readonly source: MaterializedVolume,
    private readonly destination: MaterializedVolume,
    private readonly props: K2MigrateProps,
  ) {}

  public get volume(): Volume {
    return this.destination.volume;
  }

  public configureWorkload(workload: Workload): void {
    this.source.configureWorkload?.(workload);
    this.destination.configureWorkload?.(workload);

    workload.addInitContainer({
      name: this.props.initContainerName ?? "migrate-volume",
      image: this.props.image ?? "instrumentisto/rsync-ssh:alpine",
      command: ["/bin/sh", "-c", migrationScript(this.props)],
      securityContext: rootSecurityContext(),
      volumeMounts: [
        { path: SOURCE_PATH, volume: this.source.volume, readOnly: true },
        { path: DESTINATION_PATH, volume: this.destination.volume },
      ],
    });
  }
}

const SOURCE_PATH = "/migrate/source";
const DESTINATION_PATH = "/migrate/destination";

/**
 * The marker lives on the DESTINATION so it shares the volume's lifetime: a
 * destroyed-and-recreated destination re-copies, which is the correct
 * behaviour and the reason it is not a ConfigMap or an annotation.
 */
function migrationScript(props: K2MigrateProps): string {
  const marker = `${DESTINATION_PATH}/${props.markerFile ?? ".k2-migration-complete"}`;
  return [
    "set -eu",
    `if [ -f ${marker} ]; then echo "migration already complete; skipping"; exit 0; fi`,
    `echo "copying ${SOURCE_PATH} -> ${DESTINATION_PATH}"`,
    // -a preserves ownership/modes/times, which apps notice; --delete keeps a
    // retried partial copy from leaving orphans behind.
    `rsync -a --delete --exclude ${JSON.stringify(props.markerFile ?? ".k2-migration-complete")} ${SOURCE_PATH}/ ${DESTINATION_PATH}/`,
    "sync",
    // Marker written only after rsync exits 0 and the data is flushed, so an
    // interruption anywhere above leaves the copy visibly unfinished.
    `date -u +%Y-%m-%dT%H:%M:%SZ > ${marker}`,
    'echo "migration complete"',
  ].join("\n");
}

function rootSecurityContext(): ContainerSecurityContextProps {
  // The copy must preserve ownership across UIDs the app may not run as.
  return { ensureNonRoot: false, user: 0, group: 0, readOnlyRootFilesystem: true };
}
