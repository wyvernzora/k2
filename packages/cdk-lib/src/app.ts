import * as cdk8s from "cdk8s";
import { YamlOutputType } from "cdk8s";

export interface K2AppProps extends cdk8s.AppProps {}

export class K2App extends cdk8s.App {
  constructor(props: K2AppProps = {}) {
    super({
      yamlOutputType: YamlOutputType.FILE_PER_APP,
      ...props,
    });
  }
}
