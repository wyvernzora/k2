import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { endpoint, NamespaceBoundaryPolicy, PrivateConnection, tcp } from "@k2/cilium";
import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { POCKET_ID_HTTP_PORT, POCKET_ID_LABELS } from "../lib/constants.js";

const POSTGRES_PORT = 5432;

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const pocketId = endpoint(namespace, POCKET_ID_LABELS, "pocket-id");

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-pocket-id", {
      backend: pocketId,
      ports: [tcp(POCKET_ID_HTTP_PORT)],
    });
    new PrivateConnection(this, "pocket-id-to-postgresql", {
      from: pocketId,
      to: endpoint(NEXUS_CLUSTER_NAMESPACE, { "cnpg.io/cluster": NEXUS_CLUSTER_NAME }, "nexus-postgresql"),
      ports: [tcp(POSTGRES_PORT)],
    });
  }
}
