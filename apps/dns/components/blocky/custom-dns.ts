import { Duration } from "cdk8s";

export interface CustomDnsProps {
  /**
   * Mapping of hostnames to their IP addresses.
   * If the hostname does not end in the apex domain, it is treated
   * as a subdomain of the apex. For example:
   *    apex example.com
   *    host test
   *    becomes test.example.com
   * @default {{}}
   */
  readonly records?: Record<string, string[]>;
  /**
   * TTL of custom DNS records.
   * @default 5 minutes
   */
  readonly ttl?: Duration;
}

/**
 * Configuration for custom DNS setup.
 */
export class CustomDns {
  static empty(): CustomDns {
    return new CustomDns({});
  }

  readonly records: Record<string, string[]>;
  readonly ttl: Duration;

  constructor(props: CustomDnsProps) {
    this.ttl = props.ttl || Duration.minutes(5);
    this.records = { ...props.records };
  }

  /**
   * Adds a custom DNS entry mapping a host to one or more IP addresses.
   * @param host Hostname; suffixed with apex domain name if not already.
   * @param ip IP address
   * @param ips Additional IP addresses, if any
   */
  public addRecord(host: string, ip: string, ...ips: string[]): CustomDns {
    if (!this.records[host]) {
      this.records[host] = [];
    }
    this.records[host].push(ip, ...ips);
    return this;
  }
}
