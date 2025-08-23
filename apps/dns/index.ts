import { Chart } from "cdk8s";

import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { BlockingGroup, ClientGroup, CustomDns, Blocky } from "./components/blocky/index.js";
import { K8sGateway } from "./components/gateway/index.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "dns");
  const chart = new Chart(app, "dns", { ...Namespace.of(app) });

  // Default client group and its blocking config
  const blockingGroup = new BlockingGroup({
    name: "default",
    blacklists: ["https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"],
  });
  const clientGroup = new ClientGroup({
    name: "default",
    blockingGroups: [blockingGroup],
    upstream: ["10.10.1.1"],
  });

  // Custom DNS
  const customDns = new CustomDns({
    records: {
      unifi: ["10.10.1.1"],
      roxy: ["10.10.7.1"],
      eris: ["10.10.7.2"],
      sylphy: ["10.10.7.3"],
      pve: ["10.10.7.254"],
      rumi: ["10.10.8.1"],
      k8s: ["10.10.8.2"],
    },
  });

  new K8sGateway(chart, "k8s-gateway", {
    ...ApexDomain.of(app),
    ...Namespace.of(app),
  });
  new Blocky(chart, "blocky", {
    ...ApexDomain.of(app),
    serviceIp: "10.10.10.8",
    clientGroups: [clientGroup],
    customDns,
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "dns");
};
