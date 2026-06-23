import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import {
  EndpointNetworkPolicy,
  NamespaceBoundaryPolicy,
  PrivateConnection,
  cidr,
  egress,
  endpoint,
  ingress,
  tcp,
} from "@k2/cilium";
import * as pomerium from "@k2/pomerium";
import * as postgresql from "@k2/postgresql";

import {
  FORGEJO_ALLOW_VLANS,
  FORGEJO_HTTP_PORT,
  FORGEJO_HTTP_REDIRECT_PORT,
  FORGEJO_HTTPS_PORT,
  FORGEJO_SSH_PORT,
} from "../constants.js";
import { workloads } from "../index.js";

const POCKET_ID_HTTP_PORT = 1411;
const POCKET_ID_ENDPOINT = endpoint(
  "pocket-id",
  { "app.kubernetes.io/name": "pocket-id", "app.kubernetes.io/component": "app" },
  "pocket-id",
);

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const forgejo = workloads.forgejo();
    const setup = endpoint(
      namespace,
      { "app.kubernetes.io/name": "forgejo", "app.kubernetes.io/component": "setup" },
      "forgejo-setup",
    );
    const forgejoClientCidrs = cidr.vlans(this, FORGEJO_ALLOW_VLANS);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "forgejo-network", {
      endpoint: forgejo,
      ingress: [
        ...ingress.fromCidrs(
          forgejoClientCidrs,
          tcp(FORGEJO_HTTP_REDIRECT_PORT),
          tcp(FORGEJO_HTTPS_PORT),
          tcp(FORGEJO_SSH_PORT),
        ),
        ...ingress.fromHost(tcp(FORGEJO_HTTP_REDIRECT_PORT), tcp(FORGEJO_HTTPS_PORT), tcp(FORGEJO_SSH_PORT)),
      ],
      ingressDeny: [
        {
          from: { endpoint: endpoint(namespace, {}, "forgejo-namespace") },
          ports: [tcp(FORGEJO_HTTP_PORT)],
        },
      ],
      egress: [...egress.toWorld(tcp(80), tcp(443))],
    });
    new EndpointNetworkPolicy(this, "forgejo-setup-network", {
      endpoint: setup,
      egress: [...egress.toWorld(tcp(80), tcp(443))],
    });
    new PrivateConnection(this, "forgejo-to-postgresql", {
      from: forgejo,
      ...postgresql.endpoints.nexus(),
    });
    new PrivateConnection(this, "forgejo-to-pocket-id", {
      from: forgejo,
      to: POCKET_ID_ENDPOINT,
      ports: [tcp(POCKET_ID_HTTP_PORT)],
    });
    new PrivateConnection(this, "forgejo-to-pomerium-idp", {
      from: forgejo,
      to: pomerium.workloads.proxy(),
      ports: [tcp(pomerium.POMERIUM_PROXY_HTTPS_PORT)],
    });
    new PrivateConnection(this, "forgejo-setup-to-postgresql", {
      from: setup,
      ...postgresql.endpoints.nexus(),
    });
    new PrivateConnection(this, "forgejo-setup-to-pocket-id", {
      from: setup,
      to: POCKET_ID_ENDPOINT,
      ports: [tcp(POCKET_ID_HTTP_PORT)],
    });
  }
}
