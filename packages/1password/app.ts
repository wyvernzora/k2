import { App, HelmChart } from "@k2/cdk-lib";

const TOLERATE_CONTROL_PLANE = {
  tolerations: [
    {
      key: "CriticalAddonsOnly",
      operator: "Exists",
    },
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "Exists",
      effect: "NoSchedule",
    },
    {
      key: "node-role.kubernetes.io/master",
      operator: "Exists",
      effect: "NoSchedule",
    },
  ],
};

const app = new App();
new HelmChart(app, "1password", {
  namespace: "k2-core",
  chart: "helm:https://1password.github.io/connect-helm-charts/connect@1.15.0",
  values: {
    connect: { ...TOLERATE_CONTROL_PLANE },
    operator: { create: true, ...TOLERATE_CONTROL_PLANE },
  },
});

app.synth();
