import type { PolicyEndpoint } from "./types.js";

export const NAMESPACE_LABEL = "k8s:io.kubernetes.pod.namespace";

export function endpointSelector(endpoint: PolicyEndpoint): { matchLabels: Record<string, string> } {
  return {
    matchLabels: {
      [NAMESPACE_LABEL]: endpoint.namespace,
      ...endpoint.labels,
    },
  };
}

export function namespaceSelector(namespace: string): { matchLabels: Record<string, string> } {
  return { matchLabels: { [NAMESPACE_LABEL]: namespace } };
}
