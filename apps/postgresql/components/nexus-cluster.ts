import { Chart, Size } from "cdk8s";

import { App, Namespace, Spread } from "@k2/cdk-lib";

import { Cluster, ClusterSpecResourcesLimits, ClusterSpecResourcesRequests } from "../crds/postgresql.cnpg.io.js";

export default {
  create(app: App) {
    const chart = new Chart(app, "nexus", { ...Namespace.of(app) });

    const name = "nexus";
    new Cluster(chart, "cluster", {
      metadata: { name },
      spec: {
        instances: 3,

        imageName: "ghcr.io/cloudnative-pg/postgresql:17.2-standard-bookworm",

        // Set up backing storage on replicated volumes
        // Backups will be set up to write to more durable location
        storage: {
          size: Size.gibibytes(16).asString(),
          storageClass: "longhorn",
        },
        walStorage: {
          size: Size.gibibytes(4).asString(),
          storageClass: "longhorn",
        },

        // Create DB admin user for later use in provisioner
        bootstrap: {
          initdb: {
            owner: "dba",
            postInitApplicationSql: ["ALTER ROLE dba WITH LOGIN CREATEDB CREATEROLE INHERIT;"],
            dataChecksums: true,
          },
        },

        // Spread instances across zones (i.e. physical hosts)
        topologySpreadConstraints: [
          Spread.AcrossZones({
            matchLabels: {
              "cnpg.io/cluster": name,
            },
          }),
          Spread.AcrossHosts({
            matchLabels: {
              "cnpg.io/cluster": name,
            },
          }),
        ],

        // Resource limits
        resources: {
          requests: {
            cpu: ClusterSpecResourcesRequests.fromString("500m"),
            memory: ClusterSpecResourcesRequests.fromString("1Gi"),
          },
          limits: {
            cpu: ClusterSpecResourcesLimits.fromString("2"),
            memory: ClusterSpecResourcesLimits.fromString("4Gi"),
          },
        },
      },
    });
  },
};
