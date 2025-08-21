import { Context } from "@k2/cdk-lib";
import { Duration } from "cdk8s";

export interface ArgoCdContextProps {
  readonly project: string;
  readonly namespace: string;
  readonly server: string;
  readonly repo: RepoOptions;
  readonly autoSyncPolicy?: AutoSyncPolicy;
  readonly syncOptions?: SyncOptions;
}

export interface RepoOptions {
  readonly url: string;
  readonly branch: string;
}

export type SyncOptions = Record<string, string | boolean>;

export type AutoSyncPolicy = {
  readonly retryLimit: number;
} & {
  backoff: BackoffRetryPolicy;
};

export interface BackoffRetryPolicy {
  readonly duration: Duration;
  readonly maxDuration: Duration;
  readonly factor: number;
}

export class ArgoCdContext extends Context {
  get ContextKey() {
    return "@k2/argocd:context";
  }

  readonly project: string;
  readonly namespace: string;
  readonly server: string;
  readonly repo: RepoOptions;
  readonly syncOptions: SyncOptions;
  readonly autoSyncPolicy?: AutoSyncPolicy;

  constructor(props: ArgoCdContextProps) {
    super();
    this.project = props.project;
    this.namespace = props.namespace;
    this.server = props.server;
    this.repo = props.repo;
    this.syncOptions = props.syncOptions ?? {};
    this.autoSyncPolicy = props.autoSyncPolicy;
  }
}

export function withDefaultArgoCdOptions() {
  return ArgoCdContext.with({
    project: "default",
    namespace: "k2-core",
    server: "https://kubernetes.default.svc",
    repo: {
      url: "https://github.com/wyvernzora/k2",
      branch: "deploy",
    },
    autoSyncPolicy: {
      retryLimit: 10,
      backoff: {
        duration: Duration.seconds(30),
        maxDuration: Duration.minutes(10),
        factor: 2,
      },
    },
    syncOptions: {
      CreateNamespace: true,
      ServerSideApply: true,
      ApplyOutOfSyncOnly: true,
    },
  });
}
