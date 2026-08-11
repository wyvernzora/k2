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
 *
 * Not wired into any app yet — this is the seam for the Longhorn migration.
 */
export class K2MigrateVolume extends K2Volume {
  public constructor(private readonly props: K2MigrateProps) {
    super();
  }

  public materialize(scope: Construct, id: string): MaterializedVolume {
    const source = this.props.from.materialize(scope, `${id}-source`);
    const destination = this.props.to.materialize(scope, `${id}-destination`);
    return new MigratingVolume(source, destination, this.props);
  }
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
