import type { AppResourceFunc } from "@k2/cdk-lib";

import { Blocky } from "./components/blocky/index.js";
import { CoreDnsForward } from "./components/coredns-forward.js";
import { K8sGateway } from "./components/k8s-gateway/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

const PUBLIC_DNS_SERVERS = ["1.1.1.1", "9.9.9.9"];
const BLOCKLIST_URLS = ["https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"];

export const createAppResources: AppResourceFunc = app => {
  new K8sGateway(app, "k8s-gateway", {
    publicDnsServers: PUBLIC_DNS_SERVERS,
  });
  new CoreDnsForward(app, "coredns-forward");
  new Blocky(app, "blocky", {
    publicDnsServers: PUBLIC_DNS_SERVERS,
    blocklistUrls: BLOCKLIST_URLS,
  });
  new NetworkPolicy(app, "network-policy", {
    publicDnsServers: PUBLIC_DNS_SERVERS,
    blocklistUrls: BLOCKLIST_URLS,
  });
};
