import {
  Workspace,
  WorkspacesRoot,
  findWorkspaces,
  findWorkspacesRoot,
} from "find-workspaces";
import { copyFile, mkdir } from "fs/promises";
import { resolve } from "path";
import { Node, toposort } from "./toposort";
import debug from "debug";
import { DeployOptions } from "./deploy-options";

const LOG = debug("k2:release");

// Export name of the application manifest
const APP_EXPORT = "app.k8s.yaml";

// Export name of the CRD manifest
const CRD_EXPORT = "crds.k8s.yaml";

// Default output location
const DEFAULT_OUTPUT_PATH = "deploy";

export interface Artifacts {
  readonly name: string;
  readonly namespace: string;
  readonly package: string;
  readonly exports: Set<string>;
  readonly hasManifests: boolean;
  readonly deployOptions: DeployOptions;
}

export interface CollectorProps {
  readonly dirname?: string;
  readonly output?: string;
}

/**
 * Topologically sorting glob to resolve deployable applications
 * and generate the manifests.
 */
export class Collector {
  readonly root: WorkspacesRoot;
  readonly output: string;

  readonly waves: Artifacts[][];

  constructor(props: CollectorProps = {}) {
    const wsroot = findWorkspacesRoot(props.dirname);
    if (!wsroot) {
      throw new Error(`Could not locate npm workspace root`);
    }
    this.root = wsroot;
    this.output = resolve(
      this.root.location,
      props.output || DEFAULT_OUTPUT_PATH,
    );
    // Uses toposort to determine deployment waves
    const nodes: Node<Artifacts>[] = findWorkspaces(this.root.location)!.map(
      (ws) => this.workspaceToArtifact(ws),
    );
    this.waves = toposort(nodes);
  }

  private workspaceToArtifact(ws: Workspace): Node<Artifacts> {
    const exports = new Set(Object.keys(ws.package.exports || {}));
    const hasManifests = [APP_EXPORT, CRD_EXPORT]
      .map((i) => `./${i}`)
      .some((i) => exports.has(i));
    return {
      value: {
        name: getAppName(ws.package.name),
        namespace: ws.package.namespace,
        package: ws.package.name,
        hasManifests: hasManifests,
        exports: exports,
        deployOptions: ws.package.deploy || {},
      },
      get id() {
        return ws.package.name;
      },
      get deps() {
        return getK2Dependencies(ws);
      },
    };
  }

  /**
   * Copies Kubernetes manifests exported by each package into the configured
   * output directory for further bundling.
   */
  public async copyManifests(): Promise<void> {
    const tasks = this.waves
      .flatMap(identity)
      .map((ws) => this.copyWorkspaceManifests(ws));
    await Promise.all(tasks);
  }

  // Copies exported manifests for a single workspace.
  private async copyWorkspaceManifests(ws: Artifacts): Promise<void> {
    const destination = resolve(this.output, ws.name);
    await mkdir(destination, { recursive: true });
    for (const manifest of [APP_EXPORT, CRD_EXPORT]) {
      const source = getManifestSource(ws, manifest);
      if (!source) {
        LOG(`${ws.package} does not export ${manifest}, skipping...`);
        continue;
      }
      LOG(`Copying ${source} to ${destination}`);
      await copyFile(source, resolve(destination, manifest));
    }
  }

  /**
   * Generates the root ArgoCD application manifest
   */
  public async generateRootManifest(): Promise<void> {}
}

function getK2Dependencies({ package: pkg }: Workspace): string[] {
  return [pkg.dependencies, pkg.devDependencies]
    .flatMap((i) => Object.keys(i || {}))
    .filter(isK2Dependency);
}

function isK2Dependency(name: string): boolean {
  return name.startsWith("@k2/");
}

function getAppName(name: string): string {
  if (!isK2Dependency(name)) {
    throw new Error(`Must be a @k2 scoped package to generate manifests`);
  }
  return name.substring(4);
}

function getManifestSource(ws: Artifacts, manifest: string): string | null {
  if (ws.exports.has(`./${manifest}`)) {
    return require.resolve(`${ws.package}/${manifest}`);
  }
  return null;
}

const identity = <T>(x: T) => x;
