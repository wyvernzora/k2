import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";

export class App extends base.App {
  constructor(...options: Array<AppOptionFunc>) {
    super({ yamlOutputType: YamlOutputType.FILE_PER_APP });
    options.forEach(opt => opt(this));
  }
}

// Option that gets applied to the app
export type AppOptionFunc = (app: App) => void;
