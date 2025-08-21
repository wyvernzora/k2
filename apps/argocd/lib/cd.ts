import * as crd from "../crds/argoproj.io";
import { Construct } from "constructs";
import { ArgoCdContext } from "./context";

export interface ArgoAppProps {
  path?: string;
  namespace?: string;
}

export class ContinuousDeployment extends Construct {
  readonly argo: crd.Application;

  constructor(scope: Construct, name: string, props: ArgoAppProps) {
    super(scope, name);
    const ctx = ArgoCdContext.of(this);

    this.argo = new crd.Application(this, name, {
      metadata: {
        name,
        namespace: ctx.namespace,
      },
      spec: {
        destination: {
          namespace: props.namespace,
          server: ctx.server,
        },
        project: ctx.project,
        source: {
          path: props.path ?? name,
          repoUrl: ctx.repo.url,
          targetRevision: ctx.repo.branch,
        },
        syncPolicy: {
          ...constructAutoSyncPolicy(ctx),
          ...constructSyncOptions(ctx),
        },
      },
    });
  }
}

function constructAutoSyncPolicy(ctx: ArgoCdContext): Partial<crd.ApplicationSpecSyncPolicy> {
  if (!ctx.autoSyncPolicy) {
    return {
      automated: {
        selfHeal: false,
        prune: false,
      },
    };
  }
  const backoff = ctx.autoSyncPolicy.backoff;
  return {
    automated: {
      selfHeal: true,
      prune: true,
    },
    retry: {
      limit: ctx.autoSyncPolicy.retryLimit,
      backoff: {
        duration: `${backoff.duration.toSeconds()}s`,
        maxDuration: `${backoff.maxDuration.toSeconds()}s`,
        factor: backoff.factor,
      },
    },
  };
}

function constructSyncOptions(ctx: ArgoCdContext): Partial<crd.ApplicationSpecSyncPolicy> {
  const options: string[] = Object.entries(ctx.syncOptions).map(([k, v]) => `${k}=${String(v)}`);
  return options.length ? { syncOptions: options } : {};
}
