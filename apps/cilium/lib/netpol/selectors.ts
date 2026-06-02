import type { PolicyEndpoint } from "./types.js";

export const NAMESPACE_LABEL = "k8s:io.kubernetes.pod.namespace";

interface CiliumLabelSelector {
  matchLabels: Record<string, string>;
  matchExpressions?: CiliumLabelSelectorMatchExpression[];
}

interface CiliumLabelSelectorMatchExpression {
  readonly key: string;
  readonly operator: never;
  readonly values?: string[];
}

export function endpointSelector(endpoint: PolicyEndpoint): CiliumLabelSelector {
  const selector = {
    matchLabels: {
      [NAMESPACE_LABEL]: endpoint.namespace,
      ...endpoint.labels,
    },
  };
  if (endpoint.matchExpressions === undefined || endpoint.matchExpressions.length === 0) {
    return selector;
  }
  return {
    ...selector,
    matchExpressions: endpoint.matchExpressions.map(expression => ({
      ...expression,
      operator: expression.operator as never,
    })),
  };
}

export function namespaceSelector(namespace: string): { matchLabels: Record<string, string> } {
  return { matchLabels: { [NAMESPACE_LABEL]: namespace } };
}
