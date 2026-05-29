import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

const ARGOCD_SERVER_HTTP_PORT = 8080;
const ARGOCD_SERVER_LABELS = {
  "app.kubernetes.io/instance": "argocd",
  "app.kubernetes.io/name": "argocd-server",
};

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AllowPomeriumToBackend(this, "pomerium-to-argocd-server", {
      backend: endpoint(Namespace.of(this).namespace, ARGOCD_SERVER_LABELS, "argocd-server"),
      ports: [tcp(ARGOCD_SERVER_HTTP_PORT)],
    });
  }
}
