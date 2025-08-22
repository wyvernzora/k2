import { AppResourceFunc, ArgoCDResourceFunc, K2Volume } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import { N8N } from "./components/n8n";
import { Size } from "cdk8s";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  new N8N(app, "n8n", {
    url: "https://n8n.wyvernzora.io/",
    volumes: {
      appdata: K2Volume.replicated({ size: Size.gibibytes(16) }),
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "n8n");
};
