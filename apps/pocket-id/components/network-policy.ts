import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { NamespaceBoundaryPolicy, PrivateConnection } from "@k2/cilium";
import * as postgresql from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const pocketIdHttp = endpoints.http();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-pocket-id", {
      ...pocketIdHttp,
    });
    new PrivateConnection(this, "pocket-id-to-postgresql", {
      from: pocketIdHttp.backend,
      ...postgresql.endpoints.nexus(),
    });
  }
}
