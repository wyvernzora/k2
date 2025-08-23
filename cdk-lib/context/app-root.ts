import { Context } from ".";

export class AppRoot extends Context {
  get ContextKey() {
    return "@k2/cdk-lib:app-root";
  }

  constructor(public readonly appRoot: string) {
    super();
  }
}
