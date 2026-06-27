import type { Construct } from "constructs";

import { ClusterContext, K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, egress, endpoint, NamespaceBoundaryPolicy, PrivateConnection, tcp } from "@k2/cilium";
import * as kura from "@k2/kura";
import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "@k2/postgresql";
import { AllowPomeriumToBackend, workloads as pomeriumWorkloads } from "@k2/pomerium";
import * as qbittorrent from "@k2/qbittorrent";
import * as takuhai from "@k2/takuhai";

import { N8N_ACP_AUTH_PORT, N8N_HTTP_PORT, N8N_LABELS } from "./n8n/labels.js";

const POSTGRES_PORT = 5432;
const POMERIUM_JWKS_PORT = 443;
const POMERIUM_PROXY_HTTP_PORT = 80;
const POMERIUM_PROXY_HTTPS_PORT = 443;
const POMERIUM_PROXY_HTTP_TARGET_PORT = 8080;
const POMERIUM_PROXY_HTTPS_TARGET_PORT = 8443;
const POMERIUM_FRONT_DOOR_POOL_NAME = "privileged";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const n8n = endpoint(Namespace.of(this).namespace, N8N_LABELS, "n8n");
    const kuraRest = kura.endpoints.http();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-n8n", {
      backend: n8n,
      ports: [tcp(N8N_HTTP_PORT)],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-n8n-acp-auth", {
      backend: n8n,
      ports: [tcp(N8N_ACP_AUTH_PORT)],
    });
    new PrivateConnection(this, "n8n-to-postgresql", {
      from: n8n,
      to: endpoint(NEXUS_CLUSTER_NAMESPACE, { "cnpg.io/cluster": NEXUS_CLUSTER_NAME }, "nexus-postgresql"),
      ports: [tcp(POSTGRES_PORT)],
    });
    new PrivateConnection(this, "n8n-to-takuhai", {
      from: n8n,
      to: takuhai.workloads.takuhai(),
      ports: [tcp(takuhai.TAKUHAI_HTTP_PORT)],
    });
    new PrivateConnection(this, "n8n-to-takuhai-crawler-dmhy", {
      from: n8n,
      to: takuhai.workloads.crawler(),
      ports: [tcp(takuhai.TAKUHAI_CRAWLER_PORT)],
    });
    new PrivateConnection(this, "n8n-to-kura-mcp", {
      from: n8n,
      ...kura.endpoints.mcp(),
    });
    new PrivateConnection(this, "n8n-to-kura-rest", {
      from: n8n,
      to: kuraRest.backend,
      ports: kuraRest.ports,
    });
    new PrivateConnection(this, "n8n-to-qbit-bridge", {
      from: n8n,
      ...qbittorrent.endpoints.bridge(),
    });
    new PrivateConnection(this, "n8n-to-pomerium-jwks", {
      from: n8n,
      to: pomeriumWorkloads.proxy(),
      ports: [tcp(POMERIUM_PROXY_HTTP_TARGET_PORT), tcp(POMERIUM_PROXY_HTTPS_TARGET_PORT)],
    });
    new EndpointNetworkPolicy(this, "n8n-egress", {
      endpoint: n8n,
      egress: [
        {
          to: { endpoint: pomeriumWorkloads.proxy() },
          ports: [
            tcp(POMERIUM_PROXY_HTTP_PORT),
            tcp(POMERIUM_PROXY_HTTPS_PORT),
            tcp(POMERIUM_PROXY_HTTP_TARGET_PORT),
            tcp(POMERIUM_PROXY_HTTPS_TARGET_PORT),
          ],
        },
        ...egress.toWorld(tcp(80), tcp(443)),
        ...egress.toCidrs([pomeriumFrontDoorPoolCidr(this)], tcp(POMERIUM_JWKS_PORT)),
      ],
    });
  }
}

function pomeriumFrontDoorPoolCidr(scope: Construct): string {
  const pool = ClusterContext.of(scope).config.loadBalancerPools.find(
    candidate => candidate.name === POMERIUM_FRONT_DOOR_POOL_NAME,
  );
  if (pool === undefined) {
    throw new Error(`n8n requires loadBalancerPools entry ${POMERIUM_FRONT_DOOR_POOL_NAME} for Pomerium JWKS egress`);
  }
  return pool.cidr;
}
