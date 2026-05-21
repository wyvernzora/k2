export interface ClusterArgoConfig {
  readonly namespace: string;
  readonly project: string;
  readonly server: string;
  readonly repoUrl: string;
  readonly repoBranch: string;
  /**
   * Relative path under the deploy branch root where per-app manifests live.
   * Empty string (default) places each app directly at the deploy branch root
   * as `<name>/app.k8s.yaml`. Used both for synth output and as the prefix in
   * each Argo Application's `source.path`.
   */
  readonly appsPath?: string;
  readonly autoSync: boolean;
  readonly applicationNamePrefix?: string;
  readonly syncOptions: Record<string, string | boolean>;
}
