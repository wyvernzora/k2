import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import {
  EndpointNetworkPolicy,
  NamespaceBoundaryPolicy,
  cidr,
  egress,
  endpoint,
  fqdn,
  ingress,
  tcp,
  udp,
} from "@k2/cilium";

const DNS_PORTS = [udp(53), tcp(53)];
const K8S_GATEWAY_DNS_PORTS = [udp(1053), tcp(1053)];
const HTTPS_PORTS = [tcp(443)];

export interface NetworkPolicyProps {
  readonly publicDnsServers: string[];
  readonly blocklistUrls: string[];
}

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string, props: NetworkPolicyProps) {
    super(scope, id);
    const namespace = Namespace.of(this).namespace;
    const publicDnsCidrs = props.publicDnsServers.map(address => `${address}/32`);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "blocky-dns", {
      endpoint: endpoint(namespace, {
        "app.kubernetes.io/component": "resolver",
        "app.kubernetes.io/name": "blocky",
      }),
      ingress: ingress.fromCidrs(cidr.rfc1918(), ...DNS_PORTS),
      egress: [
        ...egress.toCidrs(publicDnsCidrs, ...DNS_PORTS),
        ...egress.toDns(),
        ...egress.toFqdns(blocklistHosts(props.blocklistUrls), ...HTTPS_PORTS),
      ],
    });
    new EndpointNetworkPolicy(this, "k8s-gateway-dns", {
      endpoint: endpoint(namespace, {
        "app.kubernetes.io/instance": "k8s-gateway",
        "app.kubernetes.io/name": "k8s-gateway",
      }),
      ingress: [...ingress.fromNodes(tcp(8080)), ...ingress.fromCluster(...K8S_GATEWAY_DNS_PORTS)],
      egress: egress.toCidrs(publicDnsCidrs, ...DNS_PORTS),
    });
  }
}

function blocklistHosts(urls: string[]) {
  return urls.map(url => fqdn.name(new URL(url).hostname));
}
