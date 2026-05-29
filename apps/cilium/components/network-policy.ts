import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

const HUBBLE_UI_HTTP_PORT = 8081;
const HUBBLE_UI_LABELS = {
  "k8s-app": "hubble-ui",
};

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AllowPomeriumToBackend(this, "pomerium-to-hubble-ui", {
      backend: endpoint(Namespace.of(this).namespace, HUBBLE_UI_LABELS, "hubble-ui"),
      ports: [tcp(HUBBLE_UI_HTTP_PORT)],
    });
  }
}
