import type { AppResourceFunc, ArgoApplicationConfigFunc } from "@k2/cdk-lib";

import { NetworkPolicy } from "./components/network-policy.js";
import { Paperless } from "./components/paperless/index.js";

export const configureArgoApplication: ArgoApplicationConfigFunc = app => ({
  ignoreDifferences: [
    {
      kind: "Secret",
      name: "paperless-mcp-token",
      namespace: app.destinationNamespace,
      jsonPointers: ["/data"],
    },
  ],
});

export const createAppResources: AppResourceFunc = app => {
  new Paperless(app, "paperless");
  new NetworkPolicy(app, "network-policy");
};
