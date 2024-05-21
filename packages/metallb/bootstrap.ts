import { App, HelmChart } from "@k2/cdk-lib";

/**
 * This is a version of the chart that is only used for bootstrapping CRD
 * constructs. The actual application is provisioned at app.ts
 */
const app = new App();
new HelmChart(app, "metallb", {
  namespace: "k2-network",
  chart: "helm:https://metallb.github.io/metallb/metallb@0.14.5",
});
app.synth();
