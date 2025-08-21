import { ApexDomainContext, AppResourceFunc, ArgoCDResourceFunc } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { BlockingGroup, ClientGroup, CustomDns, Blocky } from "./components/blocky";
import { K8sGateway } from "./components/gateway";
import { Chart } from "cdk8s";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const { domain: apexDomain } = ApexDomainContext.of(app);
  const chart = new Chart(app, "dns", { namespace: "dns" });

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
    namespace: "dns",
    apexDomain,
  });
  new Blocky(chart, "blocky", {
    apexDomain,
    serviceIp: "10.10.10.8",
    clientGroups: [clientGroup],
    customDns,
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "dns", { namespace: "dns" });
};
