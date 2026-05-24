import type { PortSpec } from "./types.js";

export function tcp(port: number | string): PortSpec {
  return { protocol: "TCP", port };
}

export function udp(port: number | string): PortSpec {
  return { protocol: "UDP", port };
}
