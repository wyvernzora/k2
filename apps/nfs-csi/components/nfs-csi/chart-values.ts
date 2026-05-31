import { Scheduling } from "@k2/cdk-lib";

const NFS_CSI_STORAGE_CLASS = "nfs-csi";
const NFS_CSI_DRIVER = "nfs.csi.k8s.io";
const NFS_SHARE = "/mnt/data/volumes/nfs-csi";

export function nfsCsiValues(server: string) {
  const controllerScheduling = Scheduling.workersPreferred();
  return {
    driver: {
      name: NFS_CSI_DRIVER,
    },
    controller: {
      enableSnapshotter: false,
      affinity: controllerAffinity(controllerScheduling),
      tolerations: controllerScheduling.tolerations,
    },
    externalSnapshotter: {
      enabled: false,
      customResourceDefinitions: {
        enabled: false,
      },
    },
    volumeSnapshotClass: {
      create: false,
    },
    storageClass: {
      create: true,
      name: NFS_CSI_STORAGE_CLASS,
      annotations: {
        "storageclass.kubernetes.io/is-default-class": "false",
      },
      parameters: {
        server,
        share: NFS_SHARE,
        subDir: "${pvc.metadata.namespace}/${pvc.metadata.name}/${pv.metadata.name}",
        onDelete: "retain",
      },
      reclaimPolicy: "Retain",
      volumeBindingMode: "Immediate",
      mountOptions: ["nfsvers=4.1"],
    },
  };
}

function controllerAffinity(scheduling: ReturnType<typeof Scheduling.workersPreferred>) {
  return {
    nodeAffinity: {
      requiredDuringSchedulingIgnoredDuringExecution: linuxNodeSelector(),
      preferredDuringSchedulingIgnoredDuringExecution:
        scheduling.affinity?.nodeAffinity?.preferredDuringSchedulingIgnoredDuringExecution,
    },
  };
}

function linuxNodeSelector() {
  return {
    nodeSelectorTerms: [
      {
        matchExpressions: [linuxNodeMatchExpression()],
      },
    ],
  };
}

function linuxNodeMatchExpression() {
  return {
    key: "kubernetes.io/os",
    operator: "In",
    values: ["linux"],
  };
}
