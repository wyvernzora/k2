import { Context } from "./context.js";

export const CLUSTER_TARGETS = ["legacy", "v3"] as const;
export type ClusterTarget = (typeof CLUSTER_TARGETS)[number];

export interface ClusterConfig {
  readonly id: ClusterTarget;
  readonly deployPath: string;
  readonly apexDomain: string;
  readonly argo: ClusterArgoConfig;
}

export interface ClusterArgoConfig {
  readonly namespace: string;
  readonly project: string;
  readonly repoUrl: string;
  readonly repoBranch: string;
  readonly appsPath: string;
  readonly rootPath: string;
  readonly autoSync: boolean;
  readonly applicationNamePrefix?: string;
}

export const CLUSTERS = {
  legacy: {
    id: "legacy",
    deployPath: "deploy/legacy",
    apexDomain: "wyvernzora.io",
    argo: {
      namespace: "k2-core",
      project: "default",
      repoUrl: "https://github.com/wyvernzora/k2",
      repoBranch: "deploy",
      appsPath: "legacy/apps",
      rootPath: "legacy/argocd",
      autoSync: true,
    },
  },
  v3: {
    id: "v3",
    deployPath: "deploy/v3",
    apexDomain: "wyvernzora.io",
    argo: {
      namespace: "k2-core",
      project: "default",
      repoUrl: "https://github.com/wyvernzora/k2",
      repoBranch: "deploy",
      appsPath: "v3/apps",
      rootPath: "v3/argocd",
      autoSync: false,
      applicationNamePrefix: "v3-",
    },
  },
} satisfies Record<ClusterTarget, ClusterConfig>;

export interface AppTargetConfig {
  readonly enabled: boolean;
  readonly bootstrap?: boolean | AppTargetBootstrapConfig;
  readonly argo?: boolean | AppTargetArgoConfig;
  readonly values?: Record<string, unknown>;
}

export interface AppTargetBootstrapConfig {
  readonly order?: number;
  readonly fileName?: string;
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
    readonly order?: number;
    readonly fileName?: string;
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
    readonly bootstrapPath: string;
  };
}

export function defineDeployment(deployment: AppDeployment): AppDeployment {
  return deployment;
}

export class ClusterContext extends Context {
  get ContextKey() {
    return "@k2/cluster:context";
  }

  readonly cluster: ClusterConfig;

  constructor(cluster: ClusterConfig) {
    super();
    this.cluster = cluster;
  }
}
