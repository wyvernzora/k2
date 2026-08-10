import { k8s } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { HelmCharts, K2Chart, Namespace } from "@k2/cdk-lib";

import { CSI_DRIVER_NAME, NODE_STAGE_SECRET_NAME } from "../../constants.js";

import { democraticCsiValues } from "./chart-values.js";
import { DriverConfigSecret, NodeStageSecret } from "./secrets.js";

export const STORAGE_CLASS_NAME = "k2-iscsi";

export class DemocraticCsi extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new DriverConfigSecret(this, "driver-config");
    new NodeStageSecret(this, "node-stage");

    HelmCharts.of(this).asChart(this, "democratic-csi", "democratic-csi", democraticCsiValues());

    new k8s.KubeStorageClass(this, "storage-class", storageClassProps(Namespace.of(this).namespace));
  }
}

function storageClassProps(namespace: string): k8s.KubeStorageClassProps {
  return {
    metadata: {
      name: STORAGE_CLASS_NAME,
      annotations: {
        "storageclass.kubernetes.io/is-default-class": "false",
      },
    },
    provisioner: CSI_DRIVER_NAME,
    reclaimPolicy: "Delete",
    volumeBindingMode: "Immediate",
    allowVolumeExpansion: true,
    parameters: {
      fsType: "ext4",
      "csi.storage.k8s.io/node-stage-secret-name": NODE_STAGE_SECRET_NAME,
      "csi.storage.k8s.io/node-stage-secret-namespace": namespace,
    },
  };
}
