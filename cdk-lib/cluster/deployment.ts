import type { ClusterConfig } from "./config.js";
import type { ClusterTarget } from "./target.js";

export interface AppTargetConfig {
  readonly enabled: boolean;
  readonly bootstrap?: boolean;
  readonly argo?: boolean | AppTargetArgoConfig;
  readonly values?: Record<string, unknown>;
}

export interface AppTargetArgoConfig {
  readonly enabled?: boolean;
  readonly automated?: boolean;
  readonly prune?: boolean;
  readonly selfHeal?: boolean;
}

export interface AppDeployment {
  readonly targets: Partial<Record<ClusterTarget, boolean | AppTargetConfig>>;
}

export interface NormalizedAppTargetConfig {
  readonly enabled: boolean;
  readonly bootstrap: {
    readonly enabled: boolean;
  };
  readonly argo: {
    readonly enabled: boolean;
    readonly automated?: boolean;
    readonly prune?: boolean;
    readonly selfHeal?: boolean;
  };
  readonly values: Record<string, unknown>;
}

export interface K2SynthContext {
  readonly cluster: ClusterConfig;
  readonly appName: string;
  readonly target: ClusterTarget;
  readonly deployment: NormalizedAppTargetConfig;
  readonly output: {
    readonly appPath: string;
    readonly argoPath: string;
  };
}

export function defineDeployment(deployment: AppDeployment): AppDeployment {
  return deployment;
}
