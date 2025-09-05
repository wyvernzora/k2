import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import Tailscale from "./components/tailscale.js";
import Connector from "./components/connector.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "tailscale");
  Tailscale.create(app);
  Connector.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "tailscale");
};
