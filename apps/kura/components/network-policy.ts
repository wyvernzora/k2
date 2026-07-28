import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, PrivateConnection, egress, tcp } from "@k2/cilium";
import * as postgresql from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";
import { PrometheusPodScrape } from "@k2/prometheus";

import { KURA_RELEASE_INDEXER_METRICS_PORT } from "../constants.js";
import { endpoints } from "../index.js";

const DMHY_HOSTS = ["dmhy.org", "share.dmhy.org"];
const NYAA_HOSTS = ["nyaa.si"];
const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const TVDB_API_HOST = "api4.thetvdb.com";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const gateway = endpoints.http();
    const libraryManager = endpoints.libraryManager();
    const releaseIndexer = endpoints.releaseIndexer();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kura-gateway", {
      ...gateway,
    });
    new PrivateConnection(this, "gateway-to-library-manager", {
      from: gateway.backend,
      to: libraryManager.backend,
      ports: libraryManager.ports,
    });
    new PrivateConnection(this, "gateway-to-release-indexer", {
      from: gateway.backend,
      to: releaseIndexer.backend,
      ports: releaseIndexer.ports,
    });
    new EndpointNetworkPolicy(this, "library-manager-egress", {
      endpoint: libraryManager.backend,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toFqdns([TVDB_API_HOST], tcp(443))],
    });
    new EndpointNetworkPolicy(this, "release-indexer-source-egress", {
      endpoint: releaseIndexer.backend,
      egress: [...egress.toFqdns([...DMHY_HOSTS, ...NYAA_HOSTS], tcp(443))],
    });
    new PrivateConnection(this, "release-indexer-to-postgresql", {
      from: releaseIndexer.backend,
      ...postgresql.endpoints.nexus(),
    });
    new PrometheusPodScrape(this, "release-indexer-metrics", {
      target: releaseIndexer.backend,
      ports: [tcp(KURA_RELEASE_INDEXER_METRICS_PORT)],
      path: "/metrics",
    });
  }
}
