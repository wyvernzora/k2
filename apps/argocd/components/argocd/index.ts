import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

/**
 * Load resource-health Lua customizations from the sibling `health/`
 * directory. Each `<group>_<kind>.lua` file becomes argocd-cm key
 * `resource.customizations.health.<group>_<kind>` with the file
 * contents as the value. Entries are sorted for deterministic synth.
 */
function loadHealthCustomizations(): Record<string, string> {
  const healthDir = fileURLToPath(new URL("./health", import.meta.url));
  const out: Record<string, string> = {};
  for (const entry of readdirSync(healthDir).sort()) {
    if (!entry.endsWith(".lua")) continue;
    const suffix = entry.slice(0, -".lua".length);
    out[`resource.customizations.health.${suffix}`] = readFileSync(`${healthDir}/${entry}`, "utf8");
  }
  return out;
}

/**
 * ArgoCD Helm-chart-based component.
 *
 * - Built-in auth (dex, server.disable.auth) is disabled here; expectation is
 *   that Pomerium will own ingress authentication once app routes land.
 * - Notifications are disabled (no Slack/email targets yet).
 * - Ingress is intentionally not configured yet. ArgoCD is reachable via
 *   port-forward or LoadBalancer until its Pomerium route lands.
 * - Per-resource health customizations live as standalone Lua files under
 *   `./health/<group>_<kind>.lua` and are pulled into argocd-cm at synth time.
 *   Per-app CR customizations (e.g. CNPG DatabaseClaim/RoleClaim) come back
 *   when their owner apps are ported by dropping their .lua next to the
 *   existing argoproj.io_Application.lua.
 */
export class ArgoCD extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "argocd", "argo-cd", {
      // CRDs ship via apps/argocd/crds/crds.k8s.yaml — disable Helm-side
      // rendering to avoid duplicate manifests at apply time.
      crds: {
        install: false,
      },
      secret: {
        createSecret: false,
      },
      dex: {
        enabled: false,
      },
      notifications: {
        enabled: false,
      },
      configs: {
        params: {
          // Let an ingress controller terminate TLS once one is present.
          "server.insecure": true,
          // Disable built-in auth; ForwardAuth via Authelia will replace it.
          "server.disable.auth": true,
        },
        cm: {
          "statusbadge.enabled": true,
          "reposerver.enable.git.submodule": false,
          ...loadHealthCustomizations(),
        },
      },
    });
  }
}
