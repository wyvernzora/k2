import { K2App, HelmChart } from "@k2/cdk-lib";
import { ServiceType } from "cdk8s-plus-28";

const app = new K2App();
new HelmChart(app, "k8s-gateway", {
  namespace: "k2-network",
  chart: "helm:https://ori-edge.github.io/k8s_gateway/k8s-gateway@2.4.0",
  values: {
    domain: "wyvernzora.io",
    replicaCount: 3,
    service: {
      type: ServiceType.CLUSTER_IP,
    },
  },
});
app.synth();
