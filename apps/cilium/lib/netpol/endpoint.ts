import type { PolicyEndpoint, PolicyEndpointMatchExpression } from "./types.js";

export function endpoint(
  namespace: string,
  labels: Record<string, string>,
  name?: string,
  matchExpressions?: PolicyEndpointMatchExpression[],
): PolicyEndpoint {
  return { namespace, labels, matchExpressions, name };
}
