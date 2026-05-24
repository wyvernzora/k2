import type { Construct } from "constructs";

import { ClusterContext } from "@k2/cdk-lib";
import type { K2AppInfo } from "@k2/cdk-lib";

import { Application, type ApplicationSpecSyncPolicy } from "../crds/argoproj.io.js";

export interface ArgoApplicationProps {
  readonly name: string;
  readonly argoNamespace: string;
  readonly destinationNamespace: string;
  readonly project: string;
  readonly repoUrl: string;
  readonly repoBranch: string;
  readonly sourcePath: string;
  readonly autoSync: boolean;
}

export class ArgoApplication extends Application {
  public constructor(scope: Construct, id: string, props: ArgoApplicationProps) {
    super(scope, id, {
      metadata: {
        name: props.name,
        namespace: props.argoNamespace,
      },
      spec: {
        project: props.project,
        source: {
          repoUrl: props.repoUrl,
          targetRevision: props.repoBranch,
          path: props.sourcePath,
        },
        destination: {
          server: "https://kubernetes.default.svc",
          namespace: props.destinationNamespace,
        },
        syncPolicy: syncPolicy(props.autoSync),
      },
    });
  }
}

/**
 * Synth-time helper: construct the default Argo Application for an app.
 * Synth always calls this for every app; no per-app override hook today.
 * If an app ever needs custom sync policy / project / multi-source, add a
 * proper override hook then — don't smuggle special cases through here.
 */
export function makeDefaultArgoApplication(scope: Construct, app: K2AppInfo): ArgoApplication {
  const cluster = ClusterContext.of(scope).config;
  return new ArgoApplication(scope, app.name, {
    name: app.name,
    argoNamespace: cluster.argo.namespace,
    destinationNamespace: app.destinationNamespace,
    project: cluster.argo.project,
    repoUrl: cluster.argo.repoUrl,
    repoBranch: cluster.argo.repoBranch,
    sourcePath: app.sourcePath,
    autoSync: cluster.argo.autoSync,
  });
}

/**
 * Default sync policy. `syncOptions` apply uniformly to both manual and
 * automated syncs — most importantly `ServerSideApply=true`, without
 * which large CRDs (Argo ApplicationSet, ESO ClusterSecretStore, Cilium
 * CRDs) blow the 256 KiB annotation limit on the client-side-apply
 * `last-applied-configuration` annotation and fail to install.
 *
 * `automated:` is only set when the cluster opts into auto-sync;
 * manual-sync clusters get the syncOptions but no auto-reconcile loop.
 */
function syncPolicy(autoSync: boolean): ApplicationSpecSyncPolicy {
  return {
    automated: autoSync ? { prune: true, selfHeal: true } : undefined,
    syncOptions: ["CreateNamespace=true", "ServerSideApply=true", "ApplyOutOfSyncOnly=true"],
  };
}
