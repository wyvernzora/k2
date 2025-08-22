import { Context } from ".";

export class ApexDomainContext extends Context {
  get ContextKey() {
    return "@k2/cdk-lib:apex-domain";
  }

  constructor(public readonly apexDomain: string) {
    super();
  }

  public subdomain(name: string): string {
    return `${name}.${this.apexDomain}`;
  }
}
