import { AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import OpenWebUI from "./components/open-web-ui.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "gen-ai");
  OpenWebUI.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "gen-ai");
};
