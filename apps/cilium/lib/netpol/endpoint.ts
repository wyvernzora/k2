import type { PolicyEndpoint } from "./types.js";

export function endpoint(namespace: string, labels: Record<string, string>, name?: string): PolicyEndpoint {
  return { namespace, labels, name };
}
