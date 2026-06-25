import type { Construct } from "constructs";

import { ClusterContext, K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, fqdn, ingress, tcp, udp } from "@k2/cilium";
import { PrometheusPodScrape } from "@k2/prometheus";

import {
  CERT_SYNC_PROXMOX_LABELS,
  CERT_SYNC_TRUENAS_LABELS,
  PROXMOX_PORT,
  TRUENAS_PORT,
  proxmoxHosts,
  truenasHost,
} from "./cert-sync/index.js";

const CERT_MANAGER_METRICS_TARGETS = [
  ["controller", "cert-manager", "cert-manager-controller"],
  ["cainjector", "cainjector", "cert-manager-cainjector"],
  ["webhook", "webhook", "cert-manager-webhook"],
] as const;

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const cluster = ClusterContext.of(this).config;
    const namespace = Namespace.of(this).namespace;

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "webhook-admission-ingress", {
      endpoint: certManagerEndpoint(namespace, "webhook", "webhook", "cert-manager-webhook"),
      ingress: ingress.fromNodes(tcp(10250)),
    });
    new EndpointNetworkPolicy(this, "controller-external-egress", {
      endpoint: certManagerEndpoint(namespace, "controller", "cert-manager", "cert-manager-controller"),
      egress: [
        ...egress.toDns(),
        ...egress.toWorld(udp(53), tcp(53)),
        ...egress.toFqdns([
          fqdn.name("acme-v02.api.letsencrypt.org"),
          fqdn.name("route53.amazonaws.com"),
          fqdn.name(`sts.${awsRegion(cluster.aws?.region)}.amazonaws.com`),
        ]),
      ],
    });
    new EndpointNetworkPolicy(this, "cert-sync-proxmox-egress", {
      endpoint: endpoint(namespace, CERT_SYNC_PROXMOX_LABELS, "cert-sync-proxmox"),
      egress: egress.toCidrs(
        proxmoxHosts(this).map(host => `${host.address}/32`),
        tcp(PROXMOX_PORT),
      ),
    });
    new EndpointNetworkPolicy(this, "cert-sync-truenas-egress", {
      endpoint: endpoint(namespace, CERT_SYNC_TRUENAS_LABELS, "cert-sync-truenas"),
      egress: egress.toCidrs([`${truenasHost(this).address}/32`], tcp(TRUENAS_PORT)),
    });
    for (const [component, name, targetName] of CERT_MANAGER_METRICS_TARGETS) {
      new PrometheusPodScrape(this, `${targetName}-metrics`, {
        target: certManagerEndpoint(namespace, component, name, targetName),
        ports: [tcp(9402)],
      });
    }
  }
}

function certManagerEndpoint(namespace: string, component: string, name: string, targetName: string) {
  return endpoint(
    namespace,
    {
      "app.kubernetes.io/component": component,
      "app.kubernetes.io/instance": "cert-manager",
      "app.kubernetes.io/name": name,
    },
    targetName,
  );
}

function awsRegion(region: string | undefined): string {
  if (region === undefined || region === "") {
    throw new Error("CertManager requires clusters/v3.yaml aws.region");
  }
  return region;
}
