import type { ClusterArgoConfig } from "./argo.js";
import type { ClusterCiliumConfig } from "./cilium.js";
import type { ClusterKubernetesConfig } from "./kubernetes.js";
import type { ClusterOnePasswordConfig } from "./one-password.js";
import type { ClusterTarget } from "./target.js";

export interface ClusterConfig {
  readonly id: ClusterTarget;
  readonly deployPath: string;
  readonly apexDomain: string;
  readonly onePassword: ClusterOnePasswordConfig;
  readonly kubernetes: ClusterKubernetesConfig;
  readonly cilium?: ClusterCiliumConfig;
  readonly argo: ClusterArgoConfig;
}

export interface ClusterConfigLoadOptions {
  readonly clustersDir?: string;
  readonly schemaPath?: string;
}

export type ClusterConfigMap = Record<ClusterTarget, ClusterConfig>;
