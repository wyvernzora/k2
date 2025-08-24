import { Chart } from "cdk8s";

import { App, HelmCharts, Namespace, Spread } from "@k2/cdk-lib";

export default {
  create(app: App) {
    const CloudNativePG = HelmCharts.of(app).asConstruct("cloudnative-pg");
    const chart = new Chart(app, "cloudnative-pg", { ...Namespace.of(app) });

    // Set up the helm chart
    new CloudNativePG(chart, "op", {
      values: {
        replicaCount: 2,

        // CRDs managed separately
        crds: { create: false },

        // Watch all namespaces
        config: { clusterWide: false },

        // Set up operator permissions only
        rbac: { create: true },
        serviceAccount: { create: true },

        // Resource quotas
        resources: {
          requests: { cpu: "50m", memory: "128Mi" },
          limits: { cpu: "500m", memory: "512Mi" },
        },

        topologySpreadConstraints: [
          Spread.AcrossZones({
            matchLabels: {
              "app.kubernetes.io/name": "cloudnative-pg",
            },
          }),
          Spread.AcrossHosts({
            matchLabels: {
              "app.kubernetes.io/name": "cloudnative-pg",
            },
          }),
        ],

        // Disable monitoring for now
        monitoring: {
          podMonitorEnabled: false,
          grafanaDashboard: { create: false },
        },
      },
    });
  },
};
