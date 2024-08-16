import { App, HelmChart } from "@k2/cdk-lib";

/**
 * This is a version of the chart that is only used for bootstrapping CRD
 * constructs. The actual application is provisioned at app.ts
 */
const app = new App();
new HelmChart(app, "cert-manager", {
  namespace: "k2-core",
  chart: "helm:https://charts.jetstack.io/cert-manager@v1.15.3",
  values: {
    installCRDs: true,
  },
});
app.synth();
