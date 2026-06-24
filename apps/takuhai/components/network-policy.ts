import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, PrivateConnection, egress, tcp } from "@k2/cilium";
import * as postgresql from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

const DMHY_HOSTS = ["dmhy.org", "share.dmhy.org"];

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const takuhai = endpoints.http();
    const crawler = endpoints.crawler();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-takuhai-mcp", {
      ...takuhai,
    });
    new EndpointNetworkPolicy(this, "crawler-dmhy-egress", {
      endpoint: crawler.backend,
      egress: [...egress.toFqdns(DMHY_HOSTS, tcp(443))],
    });
    new PrivateConnection(this, "takuhai-to-postgresql", {
      from: takuhai.backend,
      ...postgresql.endpoints.nexus(),
    });
  }
}
