import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

/**
 * Upstream kubernetes-csi external-snapshotter controller: cluster-wide
 * singleton that reconciles VolumeSnapshot objects for every CSI driver
 * (democratic-csi and Longhorn both already run the per-driver snapshotter
 * sidecar). The chart's packaged CRDs land via crds/crds.k8s.yaml; the
 * deprecated validation webhook stays disabled (chart default).
 */
export class SnapshotController extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "snapshot-controller", "snapshot-controller", {});
  }
}
