import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";

export class App extends base.App {
  constructor(props: base.AppProps = {}) {
    super({
      yamlOutputType: YamlOutputType.FILE_PER_APP,
      ...props,
    });
  }
}
