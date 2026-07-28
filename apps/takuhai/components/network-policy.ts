import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, PrivateConnection, egress, tcp } from "@k2/cilium";
import * as postgresql from "@k2/postgresql";
import { PrometheusPodScrape } from "@k2/prometheus";

import { endpoints } from "../index.js";

const DMHY_HOSTS = ["dmhy.org", "share.dmhy.org"];
const NYAA_HOSTS = ["nyaa.si"];

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const takuhai = endpoints.http();
    const crawlerDmhy = endpoints.crawlerDmhy();
    const crawlerNyaa = endpoints.crawlerNyaa();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "crawler-dmhy-egress", {
      endpoint: crawlerDmhy.backend,
      egress: [...egress.toFqdns(DMHY_HOSTS, tcp(443))],
    });
    new EndpointNetworkPolicy(this, "crawler-nyaa-egress", {
      endpoint: crawlerNyaa.backend,
      egress: [...egress.toFqdns(NYAA_HOSTS, tcp(443))],
    });
    new PrivateConnection(this, "takuhai-to-postgresql", {
      from: takuhai.backend,
      ...postgresql.endpoints.nexus(),
    });
    new PrometheusPodScrape(this, "takuhai-metrics", {
      target: takuhai.backend,
      ports: takuhai.ports,
    });
    new PrometheusPodScrape(this, "crawler-dmhy-metrics", {
      target: crawlerDmhy.backend,
      ports: crawlerDmhy.ports,
    });
    new PrometheusPodScrape(this, "crawler-nyaa-metrics", {
      target: crawlerNyaa.backend,
      ports: crawlerNyaa.ports,
    });
  }
}
