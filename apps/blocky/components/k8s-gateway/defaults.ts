import type { DnsStaticRecordConfig } from "@k2/cdk-lib";

import { CustomDns } from "./custom-dns.js";

export interface DefaultCustomDnsProps {
  readonly apexDomain: string;
  readonly kubernetesApi: string;
  readonly staticRecords: DnsStaticRecordConfig[];
}

export function defaultCustomDns(props: DefaultCustomDnsProps): CustomDns {
  return new CustomDns({
    apexDomain: props.apexDomain,
    records: [
      ...props.staticRecords,
      {
        name: "k8s",
        address: props.kubernetesApi,
      },
    ],
  });
}
