import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";
import { dirname } from "path";
import { mkdir, writeFile } from "fs/promises";

export class App extends base.App {
  constructor(...options: Array<AppOptionFunc>) {
    super({ yamlOutputType: YamlOutputType.FILE_PER_APP });
    options.forEach(opt => opt(this));
  }

  async synthToFile(path: string): Promise<void> {
    const output = this.synthYaml();
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, output, "utf8");
  }
}

// Option that gets applied to the app
export type AppOptionFunc = (app: App) => void;

export type AppResourceFunc = (app: App) => void;

export type ArgoCDResourceFunc = (chart: base.Chart) => void;

export function defineAppExports<
  T extends {
    createAppResources: AppResourceFunc;
    createArgoCdResources: ArgoCDResourceFunc;
    crds?: object;
  },
>(m: T): T {
  return m;
}
