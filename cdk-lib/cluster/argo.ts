export interface ClusterArgoConfig {
  readonly namespace: string;
  readonly project: string;
  readonly server: string;
  readonly repoUrl: string;
  readonly repoBranch: string;
  readonly appsPath: string;
  readonly rootPath: string;
  readonly autoSync: boolean;
  readonly applicationNamePrefix?: string;
  readonly syncOptions: Record<string, string | boolean>;
}
