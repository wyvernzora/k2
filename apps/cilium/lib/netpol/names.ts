import type { PolicyEndpoint } from "./types.js";

export function privateConnectionDescription(from: PolicyEndpoint, to: PolicyEndpoint): string {
  return `${endpointName(from)} to ${endpointName(to)}`;
}

export function policyName(value: string): string {
  const name = value
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .toLowerCase()
    .replace(/[^a-z0-9.-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return name === "" ? "policy" : name.slice(0, 253);
}

export function endpointPolicyName(id: string, endpoint: PolicyEndpoint): string {
  return policyName(`${endpoint.namespace}-${id}`);
}

export function connectionPolicyName(id: string, from: PolicyEndpoint, to: PolicyEndpoint): string {
  return policyName(`${from.namespace}-${to.namespace}-${id}`);
}

function endpointName(endpoint: PolicyEndpoint): string {
  return endpoint.name ?? `${endpoint.namespace}-endpoint`;
}
