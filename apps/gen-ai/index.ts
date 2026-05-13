import { Size } from "cdk8s";

import { AppResourceFunc, ArgoCDResourceFunc, K2Volume, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { ClaudeRemote } from "./components/claude-remote/index.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "gen-ai");

  new ClaudeRemote(app, "claude-remote", {
    volumes: {
      state: K2Volume.replicated({ size: Size.gibibytes(2) }),
      workspace: K2Volume.replicated({ size: Size.gibibytes(10) }),
    },
  });
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "gen-ai");
};
