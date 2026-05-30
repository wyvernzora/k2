import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, NamespaceBoundaryPolicy, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { HOMER_HTTP_PORT, HOMER_LABELS } from "./homer/labels.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-homer", {
      backend: endpoint(Namespace.of(this).namespace, HOMER_LABELS, "homer"),
      ports: [tcp(HOMER_HTTP_PORT)],
    });
  }
}
