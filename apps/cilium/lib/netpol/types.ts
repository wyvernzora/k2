export interface PolicyEndpoint {
  readonly namespace: string;
  readonly labels: Record<string, string>;
  readonly name?: string;
}

export interface PortSpec {
  readonly protocol: "TCP" | "UDP";
  readonly port: number | string;
}

export interface FqdnMatch {
  readonly matchName?: string;
  readonly matchPattern?: string;
}

export interface PrivateConnectionProps {
  readonly from: PolicyEndpoint;
  readonly to: PolicyEndpoint;
  readonly ports: PortSpec[];
  readonly name?: string;
  readonly description?: string;
}

export interface EndpointNetworkPolicyProps {
  readonly endpoint: PolicyEndpoint;
  readonly ingress?: IngressRule[];
  readonly egress?: EgressRule[];
  readonly name?: string;
  readonly description?: string;
}

export interface NamespaceBoundaryPolicyProps {
  readonly namespace?: string;
  readonly name?: string;
}

export interface IngressRule {
  readonly from: IngressPeer;
  readonly ports?: PortSpec[];
}

export interface EgressRule {
  readonly to: EgressPeer;
  readonly ports?: PortSpec[];
  readonly dns?: FqdnMatch[];
}

export type IngressPeer =
  | { readonly endpoint: PolicyEndpoint }
  | { readonly entity: EntityPeer }
  | { readonly cidr: string };

export type EgressPeer =
  | { readonly endpoint: PolicyEndpoint }
  | { readonly entity: EntityPeer }
  | { readonly fqdn: FqdnMatch[] }
  | { readonly cidr: string };

export type EntityPeer = "world" | "cluster" | "host" | "remote-node" | "kube-apiserver" | "cilium-ingress";
