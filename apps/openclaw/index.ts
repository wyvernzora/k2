import { Size } from "cdk8s";

import { AppResourceFunc, ArgoCDResourceFunc, K2Volume, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { OpenClaw } from "./components/openclaw/index.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "openclaw");

  new OpenClaw(app, "openclaw", {
    volumes: {
      data: K2Volume.replicated({ size: Size.gibibytes(10) }),
    },
  });
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "openclaw");
};
