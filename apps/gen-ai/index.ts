import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import AnythingLLM from "./components/anything-llm/index.js";
import OpenWebUI from "./components/open-webui/index.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "gen-ai");
  app.use(ApexDomain, "wyvernzora.io");
  AnythingLLM.create(app);
  OpenWebUI.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "gen-ai");
};
