import { App, HelmCharts, Namespace } from "@k2/cdk-lib";

export default {
  create(app: App) {
    const DbClaimOperator = HelmCharts.of(app).asChart("dbclaim-operator");

    new DbClaimOperator(app, "dbclaim-operator", {
      ...Namespace.of(app),
      values: {
        // CRDs managed separately via apps/postgresql/crds/crds.k8s.yaml
        installCRDs: false,

        replicaCount: 2,
        leaderElection: true,

        resources: {
          requests: { cpu: "50m", memory: "64Mi" },
          limits: { cpu: "500m", memory: "256Mi" },
        },

        metrics: { enabled: false },
      },
    });
  },
};
