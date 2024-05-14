import { K2App, HelmChart } from "@k2/cdk-lib";

const app = new K2App();
new HelmChart(app, "reflector", {
  namespace: "k2-core",
  chart: "helm:https://emberstack.github.io/helm-charts/reflector@7.1.262",
  values: {
    priorityClassName: "system-cluster-critical",
  },
});
app.synth();
