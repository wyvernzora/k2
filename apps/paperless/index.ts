import { Size } from "cdk8s";

import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, K2Volume, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { Paperless } from "./components/paperless/index.js";

export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "paperless");

  new Paperless(app, "paperless", {
    host: ApexDomain.of(app).subdomain("paperless"),
    volumes: {
      data: K2Volume.replicated({ size: Size.gibibytes(4) }),
      media: K2Volume.bulk({ path: "/mnt/data/documents/vault" }),
      consume: K2Volume.bulk({ path: "/mnt/data/documents/incoming" }),
      export: K2Volume.bulk({ path: "/mnt/data/documents/exports" }),
    },
  });
};

export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "paperless");
};
