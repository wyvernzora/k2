import * as argo from "~crds/argoproj.io";
import { Construct } from "constructs";
import { Lazy } from "cdk8s";
import Debug from "debug";
import { dirname, resolve } from "path";
import { readFileSync } from "fs";
import YAML from "yaml";
import fg from "fast-glob";

const LOG = Debug("k2:app:application");

const CoreNamespace: string = "k2-core";
const DefaultProject: string = "default";
const DefaultRepo: string = "https://github.com/wyvernzora/k2";
const DefaultRevision: string = "main";

type Mutable<T> = {
  -readonly [P in keyof T]: T[P];
};
type SyncPolicy = argo.ApplicationSpecSyncPolicy;
type HelmOptions = argo.ApplicationSpecSourceHelm;
type IgnoreDifferences = argo.ApplicationSpecIgnoreDifferences;

export type ApplicationType = "cdk8s" | "helm" | "kustomize";

export interface ApplicationProps {
  readonly type: ApplicationType;
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
  public readonly type: ApplicationType;
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
          path: props.path,
          targetRevision: props.revision || DefaultRevision,
          helm: buildHelmOptions(props),
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
    this.type = props.type;
    this.dependsOn = props.dependsOn || [];
  }

  public static fromAppFile(
    scope: Construct,
    root: string,
    path: string,
  ): Application {
    const props = readApplicationProps(root, path);
    return new Application(scope, props.name, props);
  }
}

function buildHelmOptions(props: ApplicationProps): HelmOptions | undefined {
  if (props.type !== "helm") {
    LOG(
      `application ${props.name} is not a helm application, skipping helm options`,
    );
    if (props.helm) {
      throw new Error(
        `helm options provided for non-helm application ${props.name}`,
      );
    }
    return undefined;
  }
  const options: Mutable<HelmOptions> = {};
  if (props.installCRDs !== true) {
    options.skipCrds = true;
  }
  return { ...options, ...props.helm };
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
    type: determineApplicationType(dirname(abspath)),
  } as ApplicationProps;
}

function determineApplicationType(path: string): ApplicationType {
  if (fg.sync("cdk8s.{yaml,yml,json}", { cwd: path }).length > 0) {
    return "cdk8s";
  }
  if (fg.sync("Chart.{yaml,yml}", { cwd: path }).length > 0) {
    return "helm";
  }
  if (
    fg.sync(["Kustomization", "kustomization.{yaml,yml}"], { cwd: path })
      .length > 0
  ) {
    return "kustomize";
  }
  throw new Error(`unable to determine application type for ${path}`);
}
