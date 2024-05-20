import { K2App, HelmChart } from "@k2/cdk-lib";

/**
 * This is a version of the chart that is only used for bootstrapping CRD
 * constructs. The actual application is provisioned at app.ts
 */
const app = new K2App();
new HelmChart(app, "cert-manager", {
  namespace: "k2-core",
  chart: "helm:https://charts.jetstack.io/cert-manager@v1.14.5",
  values: {
    installCRDs: true,
  },
});
app.synth();