import os from "os";
import fs from "fs/promises";
import path from "path";

import fg from "fast-glob";
import pLimit from "p-limit";
import { Chart } from "cdk8s";

import { App, ApexDomain, AppRoot, CLUSTER_TARGETS, ClusterContext, HelmCharts, loadClusters } from "@k2/cdk-lib";
import type {
  AppDeployment,
  AppResourceFunc,
  ArgoCDResourceFunc,
  ClusterConfig,
  ClusterTarget,
  K2SynthContext,
  NormalizedAppTargetConfig,
} from "@k2/cdk-lib";
import * as OnePassword from "@k2/1password";
import * as ArgoCD from "@k2/argocd";

interface AppModule {
  readonly deployment?: AppDeployment;
  readonly createAppResources?: AppResourceFunc;
  readonly createArgoCdResources?: ArgoCDResourceFunc;
}

interface LoadedApp {
  readonly appName: string;
  readonly appPath: string;
  readonly mod: AppModule;
}

interface EnabledApp extends LoadedApp {
  readonly deployment: NormalizedAppTargetConfig;
}

async function main() {
  const appDirs = await fg("apps/*", { onlyDirectories: true });
  const loadedApps = await loadApps(appDirs);
  const clusterConfigs = loadClusters();
  const clusters = selectedClusters(clusterConfigs);

  const maxConcurrency = Number(process.env.MAX_CONCURRENCY) || os.cpus().length;
  const limit = pLimit(maxConcurrency);

  console.log(`Synthesizing manifests with concurrency=${maxConcurrency}`);

  for (const cluster of clusters) {
    await synthTarget(cluster, loadedApps, limit);
  }

  console.log("All synth tasks complete");
}

async function loadApps(appDirs: string[]): Promise<LoadedApp[]> {
  const apps: LoadedApp[] = [];

  for (const appPath of appDirs) {
    const appName = path.basename(appPath);
    const mod = (await import(path.resolve(appPath, "index.ts"))) as AppModule;

    if (!mod.deployment) {
      console.warn(`${appName}: missing deployment metadata; treating as legacy-only`);
    }

    apps.push({ appName, appPath, mod });
  }

  return apps;
}

async function synthTarget(
  cluster: ClusterConfig,
  loadedApps: LoadedApp[],
  limit: ReturnType<typeof pLimit>,
): Promise<void> {
  const enabledApps = enabledAppsFor(cluster, loadedApps);

  console.log(`Synthesizing ${cluster.id} manifests`);

  await cleanCluster(cluster, loadedApps);
  await Promise.all(enabledApps.map(app => limit(() => synthTargetAppManifest(cluster, app))));
  await synthTargetArgoManifest(
    cluster,
    enabledApps.filter(app => app.deployment.argo.enabled),
  );
}

/**
 * Surgical cleanup: wipe each app's output directory and the Argo bundle file
 * from a previous run, but leave sibling directories alone (so writing to
 * `deploy/` doesn't nuke `deploy/legacy/` or other mirrors).
 */
async function cleanCluster(cluster: ClusterConfig, loadedApps: LoadedApp[]): Promise<void> {
  const appsBase = appsDir(cluster);
  await Promise.all(loadedApps.map(app => fs.rm(path.join(appsBase, app.appName), { recursive: true, force: true })));
  await fs.rm(bundleFile(cluster), { force: true });
}

function appsDir(cluster: ClusterConfig): string {
  return path.join(cluster.deployPath, cluster.argo.appsPath ?? "");
}

function bundleFile(cluster: ClusterConfig): string {
  return path.join(cluster.deployPath, "app.k8s.yaml");
}

async function synthTargetAppManifest(cluster: ClusterConfig, app: EnabledApp): Promise<void> {
  const outFile = path.resolve(appsDir(cluster), app.appName, "app.k8s.yaml");
  const ctx = makeSynthContext(cluster, app, outFile, path.resolve(bundleFile(cluster)));

  if (!app.mod.createAppResources) {
    throw new Error(`${app.appName}: missing createAppResources export`);
  }
  const createAppResources = app.mod.createAppResources;

  console.log(`Synthesizing ${cluster.id}/${app.appName} CDK`);
  await createBaseApp(cluster, app.appPath)
    .use(cdkApp => createAppResources(cdkApp, ctx))
    .synthToFile(outFile);

  await copyCrdManifest(app.appPath, path.resolve(appsDir(cluster), app.appName, "crds.k8s.yaml"));
}

async function synthTargetArgoManifest(cluster: ClusterConfig, enabledApps: EnabledApp[]): Promise<void> {
  console.log(`Synthesizing ${cluster.id} ArgoCD manifest`);
  const outFile = path.resolve(bundleFile(cluster));
  const app = new App()
    .use(ClusterContext, cluster)
    .use(OnePassword.withVault(cluster.onePassword.vaultId))
    .use(ArgoCD.withClusterArgoCdOptions(cluster))
    .use(ApexDomain, cluster.apexDomain);
  const chart = new Chart(app, "argocd");

  for (const enabledApp of enabledApps) {
    synthArgoApp(chart, cluster, enabledApp, outFile);
  }

  await app.synthToFile(outFile);
}

function synthArgoApp(chart: Chart, cluster: ClusterConfig, app: EnabledApp, argoOutFile: string): void {
  if (!app.mod.createArgoCdResources) {
    throw new Error(`${app.appName}: missing createArgoCdResources export`);
  }

  const ctx = makeSynthContext(cluster, app, "", argoOutFile);
  console.log(`Synthesizing ${cluster.id}/${app.appName} ArgoCD`);
  app.mod.createArgoCdResources(chart, ctx);
}

function createBaseApp(cluster: ClusterConfig, appPath: string): App {
  return new App()
    .use(ClusterContext, cluster)
    .use(AppRoot, appPath)
    .use(HelmCharts)
    .use(OnePassword.withVault(cluster.onePassword.vaultId))
    .use(ApexDomain, cluster.apexDomain);
}

function enabledAppsFor(cluster: ClusterConfig, loadedApps: LoadedApp[]): EnabledApp[] {
  return loadedApps
    .map(app => ({
      ...app,
      deployment: normalizeDeployment(app.mod.deployment, cluster.id),
    }))
    .filter((app): app is EnabledApp => app.deployment.enabled);
}

function normalizeDeployment(deployment: AppDeployment | undefined, target: ClusterTarget): NormalizedAppTargetConfig {
  if (!deployment) {
    return target === "legacy" ? enabledConfig() : disabledConfig();
  }

  const raw = deployment.targets[target];
  if (raw == null || raw === false) {
    return disabledConfig();
  }
  if (raw === true) {
    return enabledConfig();
  }

  const bootstrap = normalizeBootstrap(raw.bootstrap);
  const argo = normalizeArgo(raw.argo);

  return {
    enabled: raw.enabled,
    bootstrap,
    argo,
    values: raw.values ?? {},
  };
}

function enabledConfig(): NormalizedAppTargetConfig {
  return {
    enabled: true,
    bootstrap: { enabled: false },
    argo: { enabled: true },
    values: {},
  };
}

function disabledConfig(): NormalizedAppTargetConfig {
  return {
    enabled: false,
    bootstrap: { enabled: false },
    argo: { enabled: false },
    values: {},
  };
}

function normalizeBootstrap(bootstrap: boolean | undefined): NormalizedAppTargetConfig["bootstrap"] {
  if (!bootstrap) {
    return { enabled: false };
  }
  return { enabled: true };
}

function normalizeArgo(
  argo:
    | boolean
    | {
        readonly enabled?: boolean;
        readonly automated?: boolean;
        readonly prune?: boolean;
        readonly selfHeal?: boolean;
      }
    | undefined,
): NormalizedAppTargetConfig["argo"] {
  if (argo === false) {
    return { enabled: false };
  }
  if (argo === true || argo == null) {
    return { enabled: true };
  }
  return {
    enabled: argo.enabled ?? true,
    automated: argo.automated,
    prune: argo.prune,
    selfHeal: argo.selfHeal,
  };
}

function makeSynthContext(cluster: ClusterConfig, app: EnabledApp, appPath: string, argoPath: string): K2SynthContext {
  return {
    cluster,
    appName: app.appName,
    target: cluster.id,
    deployment: app.deployment,
    output: {
      appPath,
      argoPath,
    },
  };
}

async function copyCrdManifest(appPath: string, dst: string): Promise<void> {
  const src = path.resolve(appPath, "crds/crds.k8s.yaml");
  try {
    await fs.copyFile(src, dst);
  } catch (err) {
    if (isNodeError(err) && err.code === "ENOENT") {
      return;
    }
    throw err;
  }
}

function selectedClusters(clusters: Record<ClusterTarget, ClusterConfig>): ClusterConfig[] {
  const raw = process.env.K2_CLUSTER ?? "all";
  if (raw === "all") {
    return CLUSTER_TARGETS.map(target => clusters[target]);
  }

  return raw.split(",").map(target => {
    if (!isClusterTarget(target)) {
      throw new Error(`Unknown K2_CLUSTER target: ${target}`);
    }
    return clusters[target];
  });
}

function isClusterTarget(target: string): target is ClusterTarget {
  return (CLUSTER_TARGETS as readonly string[]).includes(target);
}

function isNodeError(err: unknown): err is NodeJS.ErrnoException {
  return err instanceof Error && "code" in err;
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
