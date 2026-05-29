import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

const LONGHORN_UI_HTTP_PORT = 8000;
const LONGHORN_UI_LABELS = {
  app: "longhorn-ui",
};

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new AllowPomeriumToBackend(this, "pomerium-to-longhorn-ui", {
      backend: endpoint(Namespace.of(this).namespace, LONGHORN_UI_LABELS, "longhorn-ui"),
      ports: [tcp(LONGHORN_UI_HTTP_PORT)],
    });
  }
}
