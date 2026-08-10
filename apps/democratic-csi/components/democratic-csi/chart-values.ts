import { linux, only, Scheduling, type SchedulingProfile, workers } from "@k2/cdk-lib";

import { CSI_DRIVER_NAME, DRIVER_CONFIG_SECRET_NAME } from "../../constants.js";

/**
 * Values mirror the E2E-proven layout (tools/internal/workflow/e2e/
 * e2e_storage_values.go) with two deltas: the driver config comes from an
 * ESO-rendered Secret instead of inline values, and StorageClasses are
 * authored as first-class objects (chart-created ones would drag CHAP
 * credentials into values).
 *
 * Both controller and node plugin pin to workers: control-plane Pis have no
 * storage-fabric NIC and no iSCSI initiator duty.
 */
export function democraticCsiValues() {
  const scheduling = Scheduling.profile(only(workers()), only(linux()));
  return {
    csiDriver: {
      name: CSI_DRIVER_NAME,
    },
    storageClasses: [],
    volumeSnapshotClasses: [],
    driver: {
      existingConfigSecret: DRIVER_CONFIG_SECRET_NAME,
      // The rendered config lives in the secret; the chart still reads the
      // driver name from values to decide iSCSI host mounts (_helpers.tpl
      // mount-iscsi).
      config: {
        driver: "zfs-generic-iscsi",
      },
    },
    controller: {
      affinity: { nodeAffinity: scheduling.affinity?.nodeAffinity },
      tolerations: scheduling.tolerations,
    },
    node: {
      affinity: { nodeAffinity: scheduling.affinity?.nodeAffinity },
      tolerations: nodeTolerations(scheduling),
      driver: {
        iscsiDirHostPath: "/etc/iscsi",
        iscsiDirHostPathType: "DirectoryOrCreate",
      },
    },
  };
}

function nodeTolerations(scheduling: SchedulingProfile) {
  return scheduling.tolerations ?? [];
}
