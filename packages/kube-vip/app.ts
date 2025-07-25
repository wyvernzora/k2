import { App, HelmChart } from "@k2/cdk-lib";

const app = new App();
new HelmChart(app, "kube-vip", {
  namespace: "k2-network",
  chart: "helm:https://kube-vip.github.io/helm-charts/kube-vip@0.6.6",
  values: {
    config: {
      address: "10.10.8.2",
    },
    env: {
      cp_enable: "true",
      svc_enable: "false",
      vip_leaderelection: "true",
    },
    resources: {
      limits: {
        cpu: "200m",
        memory: "200Mi",
      },
      requests: {
        cpu: "50m",
        memory: "100Mi",
      },
    },
    affinity: {
      nodeAffinity: {
        requiredDuringSchedulingIgnoredDuringExecution: {
          nodeSelectorTerms: [
            {
              matchExpressions: [
                {
                  key: "node-role.kubernetes.io/master",
                  operator: "Exists",
                },
              ],
            },
            {
              matchExpressions: [
                {
                  key: "node-role.kubernetes.io/control-plane",
                  operator: "Exists",
                },
              ],
            },
          ],
        },
      },
    },
  },
});
app.synth();
