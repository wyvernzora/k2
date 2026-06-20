import type { CidrTarget, EgressRule, FqdnMatch, PolicyEndpoint, PortSpec } from "./types.js";
import { fqdnMatch } from "./fqdn-match.js";
import { tcp, udp } from "./port-spec.js";

const KUBE_DNS_ENDPOINT: PolicyEndpoint = {
  name: "kube-dns",
  namespace: "kube-system",
  labels: { "k8s-app": "kube-dns" },
};

export const egress = {
  toDns(...matches: Array<string | FqdnMatch>): EgressRule[] {
    return [
      {
        to: { endpoint: KUBE_DNS_ENDPOINT },
        ports: [udp(53), tcp(53)],
        dns: matches.length > 0 ? matches.map(fqdnMatch) : [{ matchPattern: "*" }],
      },
    ];
  },
  toWorld(...ports: PortSpec[]): EgressRule[] {
    return [{ to: { entity: "world" }, ports: portList(ports) }];
  },
  toCidrs(cidrs: string[], ...ports: PortSpec[]): EgressRule[] {
    return cidrs.map(cidr => ({ to: { cidr }, ports: portList(ports) }));
  },
  toCidrTarget(target: CidrTarget): EgressRule[] {
    return egress.toCidrs(target.cidrs, ...target.ports);
  },
  toFqdns(matches: Array<string | FqdnMatch>, ...ports: PortSpec[]): EgressRule[] {
    return [{ to: { fqdn: matches.map(fqdnMatch) }, ports: ports.length > 0 ? ports : [tcp(443)] }];
  },
  toKubeApiServer(...ports: PortSpec[]): EgressRule[] {
    return [{ to: { entity: "kube-apiserver" }, ports: portList(ports) }];
  },
};

function portList(ports: PortSpec[]): PortSpec[] | undefined {
  return ports.length > 0 ? ports : undefined;
}
