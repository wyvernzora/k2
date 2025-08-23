import { Size } from "cdk8s";

import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, K2Volume } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { HomeAutomation } from "./components";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const { apexDomain } = ApexDomain.of(app);
  new HomeAutomation(app, "home-automation", {
    namespace: "home-automation",
    hostname: `ha.${apexDomain}`,
    coordinator: "tcp://10.10.229.62:6638",
    volumes: {
      mosquitto: {
        data: K2Volume.replicated({
          size: Size.gibibytes(1),
        }),
      },
      zigbee2mqtt: {
        data: K2Volume.replicated({
          size: Size.gibibytes(1),
        }),
      },
      homeAssistant: {
        config: K2Volume.replicated({
          size: Size.gibibytes(1),
        }),
      },
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "home-automation");
};
