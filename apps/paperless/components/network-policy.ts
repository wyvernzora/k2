import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, PrivateConnection, tcp } from "@k2/cilium";
import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { PAPERLESS_HTTP_PORT, PAPERLESS_LABELS, PAPERLESS_MCP_PORT } from "./paperless/labels.js";

const POSTGRES_PORT = 5432;
const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const COMPONENT_LABEL = "app.kubernetes.io/component";
const SETUP_COMPONENT = "setup";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const paperless = endpoint(namespace, PAPERLESS_LABELS, "paperless");
    const sameNamespaceExceptSetup = endpoint(namespace, {}, "paperless-namespace-except-setup", [
      { key: COMPONENT_LABEL, operator: "NotIn", values: [SETUP_COMPONENT] },
    ]);
    const sameNamespace = endpoint(namespace, {}, "paperless-namespace");

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "paperless-http-ingress-deny", {
      endpoint: paperless,
      ingressDeny: [{ from: { endpoint: sameNamespaceExceptSetup }, ports: [tcp(PAPERLESS_HTTP_PORT)] }],
    });
    new EndpointNetworkPolicy(this, "paperless-mcp-ingress-deny", {
      endpoint: paperless,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: [tcp(PAPERLESS_MCP_PORT)] }],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-paperless", {
      backend: paperless,
      ports: [tcp(PAPERLESS_HTTP_PORT)],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-paperless-mcp", {
      backend: paperless,
      ports: [tcp(PAPERLESS_MCP_PORT)],
    });
    new PrivateConnection(this, "paperless-to-postgresql", {
      from: paperless,
      to: endpoint(NEXUS_CLUSTER_NAMESPACE, { "cnpg.io/cluster": NEXUS_CLUSTER_NAME }, "nexus-postgresql"),
      ports: [tcp(POSTGRES_PORT)],
    });
    new EndpointNetworkPolicy(this, "paperless-egress", {
      endpoint: paperless,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049))],
    });
  }
}
