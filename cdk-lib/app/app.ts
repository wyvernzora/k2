import { App, type AppProps, YamlOutputType } from "cdk8s";

import { contextOption, type AppOption, type Context, type ContextClass } from "../context/base.js";

export interface K2AppProps extends Omit<AppProps, "yamlOutputType"> {
  readonly options?: AppOption[];
}

export class K2App extends App {
  public constructor(props: K2AppProps = {}) {
    const { options, ...appProps } = props;
    super({
      yamlOutputType: YamlOutputType.FILE_PER_APP,
      ...appProps,
    });

    for (const option of options ?? []) {
      option(this);
    }
  }

  public use(option: AppOption): this;
  public use<T extends Context, Args extends unknown[]>(Ctor: ContextClass<T, Args>, ...args: Args): this;
  public use<T extends Context, Args extends unknown[]>(
    optionOrCtor: AppOption | ContextClass<T, Args>,
    ...args: Args
  ): this {
    if ("contextKey" in optionOrCtor) {
      contextOption(optionOrCtor, ...args)(this);
    } else {
      optionOrCtor(this);
    }
    return this;
  }
}
