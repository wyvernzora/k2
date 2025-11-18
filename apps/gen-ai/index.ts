import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import AnythingLLM from "./components/anything-llm/index.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "gen-ai");
  app.use(ApexDomain, "ai.wyvernzora.io");
  AnythingLLM.create(app);
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "gen-ai");
};
