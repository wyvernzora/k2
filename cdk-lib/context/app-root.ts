import { Context } from "../context.js";

export class AppRoot extends Context {
  get ContextKey() {
    return "@k2/cdk-lib:app-root";
  }

  constructor(public readonly appRoot: string) {
    super();
  }
}
