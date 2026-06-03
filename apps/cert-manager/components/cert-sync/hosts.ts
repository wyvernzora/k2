import type { IConstruct } from "constructs";

import { ClusterContext } from "@k2/cdk-lib";

export interface CertSyncHost {
  readonly name: string;
  readonly address: string;
}

export const PROXMOX_HOST_NAMES = ["roxy", "eris", "sylphy"] as const;
export const PROXMOX_PORT = 8006;
export const TRUENAS_HOST_NAME = "rumi";
export const TRUENAS_PORT = 443;

export function proxmoxHosts(scope: IConstruct): CertSyncHost[] {
  return PROXMOX_HOST_NAMES.map(name => ({
    name,
    address: staticRecordAddress(scope, name),
  }));
}

export function truenasHost(scope: IConstruct): CertSyncHost {
  return {
    name: TRUENAS_HOST_NAME,
    address: staticRecordAddress(scope, TRUENAS_HOST_NAME),
  };
}

function staticRecordAddress(scope: IConstruct, name: string): string {
  const records = ClusterContext.of(scope).config.dns.staticRecords;
  const record = records.find(item => item.name === name);
  if (record === undefined) {
    throw new Error(`cert-sync requires clusters/v3.yaml dns.staticRecords entry "${name}"`);
  }
  return record.address;
}
