import * as argo from "@k2/crds/argoproj.io";
import { Construct } from "constructs";
import { Lazy } from "cdk8s";
import Debug from "debug";
import { dirname, resolve } from "path";
import { copyFileSync, mkdirSync, readFileSync } from "fs";
import YAML from "yaml";

const LOG = Debug("k2:app:application");

const CoreNamespace: string = "k2-core";
const DefaultProject: string = "default";
const DefaultRepo: string = "https://github.com/wyvernzora/k2";
const DefaultRevision: string = "deploy";

type Mutable<T> = {
  -readonly [P in keyof T]: T[P];
};
type SyncPolicy = argo.ApplicationSpecSyncPolicy;
type HelmOptions = argo.ApplicationSpecSourceHelm;
type IgnoreDifferences = argo.ApplicationSpecIgnoreDifferences;

export interface ApplicationProps {
  readonly name: string;
  readonly namespace: string;
  readonly repo?: string;
  readonly path: string;
  readonly revision?: string;
  readonly dependsOn?: Array<string>;
  readonly autoSync?: boolean;
  readonly installCRDs?: boolean;
  readonly allowRetry?: boolean;
  readonly ignoreDifferences?: Array<IgnoreDifferences>;
  readonly helm?: HelmOptions;
}

export class Application extends argo.Application {
  public syncWave: number = 0;
  public readonly name: string;
  public readonly dependsOn: Array<string>;

  constructor(scope: Construct, id: string, props: ApplicationProps) {
    super(scope, id, {
      metadata: {
        name: props.name,
        namespace: CoreNamespace,
        annotations: {
          "argocd.argoproj.io/sync-wave": Lazy.any({
            produce: () => `${this.syncWave}`,
          }),
        },
      },
      spec: {
        project: DefaultProject,
        source: {
          repoUrl: props.repo || DefaultRepo,
          path: props.name,
          targetRevision: props.revision || DefaultRevision,
        },
        destination: {
          server: "https://kubernetes.default.svc",
          namespace: props.namespace,
        },
        ignoreDifferences: props.ignoreDifferences,
        syncPolicy: buildSyncPolicy(props),
      },
    });
    this.name = props.name;
    this.dependsOn = props.dependsOn || [];
  }

  public static fromAppFile(
    scope: Construct,
    root: string,
    path: string,
  ): Application {
    const props = readApplicationProps(root, path);
    copyApplicationManifest(root, props.name);
    return new Application(scope, props.name, props);
  }
}

function buildSyncPolicy(props: ApplicationProps): SyncPolicy {
  const policy: Mutable<SyncPolicy> = {
    syncOptions: [
      "CreateNamespace=true",
      "ServerSideApply=true",
      "ApplyOutOfSyncOnly=true",
    ],
  };
  if (props.autoSync !== false) {
    policy.automated = {
      prune: true,
      selfHeal: true,
    };
  }
  if (props.allowRetry !== false) {
    policy.retry = {
      limit: 10,
      backoff: {
        duration: "30s",
        maxDuration: "10m",
        factor: 2,
      },
    };
  }
  return policy;
}

function readApplicationProps(root: string, path: string): ApplicationProps {
  const abspath = resolve(root, path);
  const data = readFileSync(abspath, "utf8");
  LOG(`reading application props from ${path}`);
  return {
    ...YAML.parse(data),
    path: dirname(path),
  } as ApplicationProps;
}

function copyApplicationManifest(root: string, name: string): void {
  const from = require.resolve(`@k2/${name}/manifest`);
  const to = resolve(root, "deploy/dist", name);
  mkdirSync(to, { recursive: true });
  copyFileSync(from, resolve(to, "manifest.k8s.yaml"));
}
