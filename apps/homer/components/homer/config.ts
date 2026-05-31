import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import { stringify } from "yaml";

import { ApexDomain } from "@k2/cdk-lib";

const CONFIG_MAP_NAME = "homer-config";
const CONFIG_KEY = "config.yml";
const SELFHST_ICON_BASE_URL = "https://cdn.jsdelivr.net/gh/selfhst/icons/svg";
const K2_LOGO_URL = "https://raw.githubusercontent.com/wyvernzora/k2/main-v3/.github/assets/k2.png";

export class HomerConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string) {
    const config = renderHomerConfig(ApexDomain.of(scope));
    super(scope, id, {
      metadata: {
        name: CONFIG_MAP_NAME,
      },
      data: {
        [CONFIG_KEY]: config,
      },
    });
    this.checksum = createHash("sha256").update(config).digest("hex");
  }
}

function renderHomerConfig(apex: ApexDomain): string {
  return stringify(homerConfig(apex));
}

function homerConfig(apex: ApexDomain) {
  return {
    title: "K2",
    subtitle: "Homelab",
    documentTitle: "K2 Home",
    logo: K2_LOGO_URL,
    header: true,
    footer: false,
    columns: "auto",
    connectivityCheck: true,
    services: [
      {
        name: "Cluster",
        items: [
          service("Argo CD", "GitOps control plane", "argo-cd", apex.subdomain("argo")),
          service("Hubble", "Cilium observability", "cilium", apex.subdomain("hubble")),
          service("Longhorn", "Storage dashboard", "rancher-longhorn-dark", apex.subdomain("longhorn")),
        ],
      },
      {
        name: "Identity",
        items: [
          service("Pocket ID", "OIDC identity provider", "pocket-id", apex.subdomain("id")),
          service("Pomerium Login", "Authentication portal", "pomerium", apex.subdomain("login")),
        ],
      },
      {
        name: "Media",
        items: [service("Kura", "Anime library manager", "kura", apex.subdomain("kura"))],
      },
      {
        name: "Infrastructure",
        items: [
          service("Proxmox VE VIP", "Virtualization cluster", "proxmox", apex.subdomain("pve")),
          service("TrueNAS Scale", "Storage appliance", "truenas-scale", apex.subdomain("rumi")),
          service("UniFi Console", "Network controller", "ubiquiti-unifi", "unifi.ui.com"),
        ],
      },
    ],
  };
}

function service(name: string, subtitle: string, icon: string | undefined, host: string) {
  return {
    name,
    subtitle,
    ...(icon === undefined ? {} : { logo: `${SELFHST_ICON_BASE_URL}/${icon}.svg` }),
    url: `https://${host}`,
  };
}
