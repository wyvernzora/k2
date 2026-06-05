import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AllowPomeriumToBackend(this, "pomerium-to-argocd-server", {
      ...endpoints.http(),
    });
  }
}
