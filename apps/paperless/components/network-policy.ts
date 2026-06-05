import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, PrivateConnection, tcp } from "@k2/cilium";
import * as postgresql from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const COMPONENT_LABEL = "app.kubernetes.io/component";
const SETUP_COMPONENT = "setup";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const paperlessHttp = endpoints.http();
    const paperlessMcp = endpoints.mcp();
    const sameNamespaceExceptSetup = endpoint(namespace, {}, "paperless-namespace-except-setup", [
      { key: COMPONENT_LABEL, operator: "NotIn", values: [SETUP_COMPONENT] },
    ]);
    const sameNamespace = endpoint(namespace, {}, "paperless-namespace");

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "paperless-http-ingress-deny", {
      endpoint: paperlessHttp.backend,
      ingressDeny: [{ from: { endpoint: sameNamespaceExceptSetup }, ports: paperlessHttp.ports }],
    });
    new EndpointNetworkPolicy(this, "paperless-mcp-ingress-deny", {
      endpoint: paperlessMcp.backend,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: paperlessMcp.ports }],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-paperless", {
      ...paperlessHttp,
    });
    new AllowPomeriumToBackend(this, "pomerium-to-paperless-mcp", {
      ...paperlessMcp,
    });
    new PrivateConnection(this, "paperless-to-postgresql", {
      from: paperlessHttp.backend,
      ...postgresql.endpoints.nexus(),
    });
    new EndpointNetworkPolicy(this, "paperless-egress", {
      endpoint: paperlessHttp.backend,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049))],
    });
  }
}
