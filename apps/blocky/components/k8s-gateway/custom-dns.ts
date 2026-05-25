import type { DnsStaticRecordConfig } from "@k2/cdk-lib";

export interface CustomDnsProps {
  readonly apexDomain: string;
  readonly records: DnsStaticRecordConfig[];
  readonly ttlSeconds?: number;
}

export class CustomDns {
  private readonly apexDomain: string;
  private readonly records: DnsStaticRecordConfig[];
  private readonly ttlSeconds: number;

  public constructor(props: CustomDnsProps) {
    this.apexDomain = props.apexDomain;
    this.records = props.records;
    this.ttlSeconds = props.ttlSeconds ?? 60;
  }

  public toHostsPluginBlock(): string {
    const lines = [`ttl ${this.ttlSeconds}`];
    for (const record of this.records) {
      lines.push(`${record.address} ${this.fqdn(record.name)}`);
    }
    lines.push("fallthrough");
    return lines.join("\n");
  }

  private fqdn(host: string): string {
    return host.endsWith(this.apexDomain) ? host : `${host}.${this.apexDomain}`;
  }
}
