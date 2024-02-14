import { Application } from "./application";
import fg from "fast-glob";
import { Construct } from "constructs";
import Debug from "debug";

const LOG = Debug("k2:app:collector");

export interface CollectorProps {
  readonly root: string;
}

export class Collector extends Construct {
  private readonly root: string;
  private readonly _applications: Record<string, Application>;

  constructor(scope: Construct, id: string, props: CollectorProps) {
    super(scope, id);
    this.root = props.root;
    this._applications = {};
  }

  public get applications(): Application[] {
    return Object.values(this._applications).sort(
      (a, b) => a.syncWave - b.syncWave,
    );
  }

  public addApplication(app: Application): void {
    if (app.name in this._applications) {
      LOG(this._applications);
      throw new Error(`application name ${app.name} already exists`);
    }
    this._applications[app.name] = app;
  }

  private globApplications(): void {
    const appFiles = fg.sync("**/k2-app.yaml", { cwd: this.root });
    LOG(`found ${appFiles.length} applications`, appFiles);
    for (const appFile of appFiles) {
      const app = Application.fromAppFile(this, this.root, appFile);
      LOG(`adding ${app.name} of type ${app.type} from ${appFile}`);
      this.addApplication(app);
    }
  }

  public collect(): void {
    this.globApplications();

    const syncWaves: Application[][] = [];
    const resolved: Set<string> = new Set();

    const findNextWave = () => {
      const results: Application[] = [];
      for (const app of Object.values(this._applications)) {
        if (resolved.has(app.name)) {
          continue;
        }
        if (app.dependsOn.every((dep) => resolved.has(dep))) {
          LOG(`adding ${app.name} to next wave`);
          results.push(app);
        }
      }
      if (results.length === 0) {
        throw new Error("circular or missing dependency detected");
      }
      return results;
    };

    while (resolved.size < Object.keys(this._applications).length) {
      const nextWave = findNextWave();
      syncWaves.push(nextWave);
      for (const app of nextWave) {
        resolved.add(app.name);
      }
    }

    for (const [index, wave] of syncWaves.entries()) {
      for (const app of wave) {
        LOG(`wave ${index}: ${app.name}`);
        app.syncWave = index;
      }
    }
  }

  public printWaves(): void {
    for (const app of this.applications) {
      console.error(`Wave ${app.syncWave} - ${app.name}`);
    }
  }
}
