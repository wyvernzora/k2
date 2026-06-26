import type { AppResourceFunc } from "@k2/cdk-lib";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { Qbittorrent } from "./components/qbittorrent/index.js";
import { FLOOD_HTTP_PORT, QBITTORRENT_LABELS, QBIT_BRIDGE_PORT } from "./constants.js";

export * from "./lib/n8n-custom-nodes.js";

const QBITTORRENT_NAMESPACE = "qbittorrent";

export const endpoints = {
  web(): BackendTarget {
    return { backend: qbittorrentEndpoint(), ports: [tcp(FLOOD_HTTP_PORT), tcp(QBIT_BRIDGE_PORT)] };
  },

  bridge(): PrivateConnectionTarget {
    return {
      to: qbittorrentEndpoint(),
      ports: [tcp(QBIT_BRIDGE_PORT)],
    };
  },

  mcp(): PrivateConnectionTarget {
    return endpoints.bridge();
  },
};

function qbittorrentEndpoint(): PolicyEndpoint {
  return endpoint(QBITTORRENT_NAMESPACE, QBITTORRENT_LABELS, "qbittorrent");
}

export const createAppResources: AppResourceFunc = app => {
  new Qbittorrent(app, "qbittorrent");
  new NetworkPolicy(app, "network-policy");
};
