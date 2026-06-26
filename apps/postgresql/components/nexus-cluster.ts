import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { K2Chart, only, Scheduling, TopologySpread, workers } from "@k2/cdk-lib";

import {
  Cluster,
  ClusterSpecResourcesLimits,
  ClusterSpecResourcesRequests,
  type ClusterSpec,
  type ClusterProps,
} from "../crds/postgresql.cnpg.io.js";
import { NEXUS_CLUSTER_NAME } from "../lib/nexus.js";

const STORAGE_CLASS = "longhorn";
const ResourceLimits = ClusterSpecResourcesLimits;
const ResourceRequests = ClusterSpecResourcesRequests;

export class NexusCluster extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new NexusPostgresCluster(this, "cluster");
  }
}

class NexusPostgresCluster extends Cluster {
  public constructor(scope: Construct, id: string) {
    super(scope, id, nexusClusterProps());
  }
}

function nexusClusterProps(): ClusterProps {
  return {
    metadata: nexusClusterMetadata(),
    spec: nexusClusterSpec(),
  };
}

function nexusClusterMetadata(): ClusterProps["metadata"] {
  return {
    name: NEXUS_CLUSTER_NAME,
    annotations: {
      "cnpg.wyvernzora.io/allowed-claim-namespaces": "paperless,pocket-id,pomerium,n8n,forgejo,takuhai",
    },
  };
}

function nexusClusterSpec(): ClusterSpec {
  const name = NEXUS_CLUSTER_NAME;
  return {
    instances: 3,
    imageName: "ghcr.io/cloudnative-pg/postgresql:17.9-standard-bookworm",
    enableSuperuserAccess: true,
    storage: volumeStorage(),
    walStorage: volumeStorage(),
    affinity: Scheduling.profile(only(workers())).affinity as NonNullable<ClusterSpec["affinity"]>,
    bootstrap: bootstrap(),
    topologySpreadConstraints: clusterTopologySpread(name),
    resources: clusterResources(),
  };
}

function volumeStorage(): NonNullable<ClusterSpec["storage"]> {
  return {
    size: Size.gibibytes(4).asString(),
    storageClass: STORAGE_CLASS,
  };
}

function bootstrap(): NonNullable<ClusterSpec["bootstrap"]> {
  return {
    initdb: initdbBootstrap(),
  };
}

function initdbBootstrap(): NonNullable<NonNullable<ClusterSpec["bootstrap"]>["initdb"]> {
  return {
    owner: "dba",
    postInitApplicationSql: ["ALTER ROLE dba WITH LOGIN CREATEDB CREATEROLE INHERIT;"],
    dataChecksums: true,
  };
}

function clusterTopologySpread(name: string): NonNullable<ClusterSpec["topologySpreadConstraints"]> {
  const matchLabels = clusterMatchLabels(name);
  return [TopologySpread.acrossZones({ matchLabels }), TopologySpread.acrossHosts({ matchLabels })];
}

function clusterMatchLabels(name: string): Record<string, string> {
  return {
    "cnpg.io/cluster": name,
  };
}

function clusterResources(): NonNullable<ClusterSpec["resources"]> {
  return {
    requests: {
      cpu: ResourceRequests.fromString("500m"),
      memory: ResourceRequests.fromString("1Gi"),
    },
    limits: {
      cpu: ResourceLimits.fromString("2"),
      memory: ResourceLimits.fromString("4Gi"),
    },
  };
}
