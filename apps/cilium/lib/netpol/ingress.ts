import type { IngressRule, PortSpec } from "./types.js";

export const ingress = {
  fromWorld(...ports: PortSpec[]): IngressRule[] {
    return [{ from: { entity: "world" }, ports: portList(ports) }];
  },
  fromCidrs(cidrs: string[], ...ports: PortSpec[]): IngressRule[] {
    return cidrs.map(cidr => ({ from: { cidr }, ports: portList(ports) }));
  },
  fromKubeApiServer(...ports: PortSpec[]): IngressRule[] {
    return [{ from: { entity: "kube-apiserver" }, ports: portList(ports) }];
  },
  fromHost(...ports: PortSpec[]): IngressRule[] {
    return [{ from: { entity: "host" }, ports: portList(ports) }];
  },
  fromRemoteNode(...ports: PortSpec[]): IngressRule[] {
    return [{ from: { entity: "remote-node" }, ports: portList(ports) }];
  },
  fromNodes(...ports: PortSpec[]): IngressRule[] {
    return [...ingress.fromHost(...ports), ...ingress.fromRemoteNode(...ports)];
  },
};

function portList(ports: PortSpec[]): PortSpec[] | undefined {
  return ports.length > 0 ? ports : undefined;
}
