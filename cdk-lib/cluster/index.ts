import { readFile } from "node:fs/promises";

import { parse } from "yaml";

import type { ClusterConfig } from "./config.js";

const CIDR_PATTERN = /^(?:\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;

export async function loadClusterConfig(path = "clusters/v3.yaml"): Promise<ClusterConfig> {
  const raw = await readFile(path, "utf8");
  const config: unknown = parse(raw);
  validateClusterConfig(config, path);
  return config;
}

function validateClusterConfig(value: unknown, path: string): asserts value is ClusterConfig {
  const root = requireObject(value, path);
  requireConst(root, "id", "v3", path);
  requireNonEmptyString(root, "apexDomain", path);

  if (root.aws !== undefined) {
    const aws = requireObject(root.aws, `${path}.aws`);
    requireAwsAccountId(aws, "accountId", `${path}.aws`);
    requireNonEmptyString(aws, "region", `${path}.aws`);
    if (aws.oidcIssuer !== undefined) {
      const oidcIssuer = requireObject(aws.oidcIssuer, `${path}.aws.oidcIssuer`);
      requireHttpsUrl(oidcIssuer, "url", `${path}.aws.oidcIssuer`);
      requireHttpsUrl(oidcIssuer, "jwksUri", `${path}.aws.oidcIssuer`);
    }
  }

  const op = requireObject(root.onePassword, `${path}.onePassword`);
  requireNonEmptyString(op, "vault", `${path}.onePassword`);

  const kubernetes = requireObject(root.kubernetes, `${path}.kubernetes`);
  requireNonEmptyString(kubernetes, "api", `${path}.kubernetes`);
  requireNonEmptyString(kubernetes, "dns", `${path}.kubernetes`);
  requireNonEmptyString(kubernetes, "domain", `${path}.kubernetes`);
  const subnets = requireObject(kubernetes.subnets, `${path}.kubernetes.subnets`);
  requireCidr(subnets, "pods", `${path}.kubernetes.subnets`);
  requireCidr(subnets, "services", `${path}.kubernetes.subnets`);

  const network = requireObject(root.network, `${path}.network`);
  const vlans = requireArray(network.vlans, `${path}.network.vlans`);
  vlans.forEach((entry, index) => {
    const vlan = requireObject(entry, `${path}.network.vlans[${index}]`);
    requireNonEmptyString(vlan, "name", `${path}.network.vlans[${index}]`);
    requireInteger(vlan, "id", `${path}.network.vlans[${index}]`, 1, 4094);
    requireCidr(vlan, "cidr", `${path}.network.vlans[${index}]`);
  });

  const dns = requireObject(root.dns, `${path}.dns`);
  const staticRecords = requireArray(dns.staticRecords, `${path}.dns.staticRecords`);
  staticRecords.forEach((entry, index) => {
    const record = requireObject(entry, `${path}.dns.staticRecords[${index}]`);
    requireNonEmptyString(record, "name", `${path}.dns.staticRecords[${index}]`);
    requireIpv4Address(record, "address", `${path}.dns.staticRecords[${index}]`);
  });

  const argo = requireObject(root.argo, `${path}.argo`);
  requireNonEmptyString(argo, "namespace", `${path}.argo`);
  requireNonEmptyString(argo, "project", `${path}.argo`);
  requireHttpsUrl(argo, "repoUrl", `${path}.argo`);
  requireNonEmptyString(argo, "repoBranch", `${path}.argo`);
  requireBoolean(argo, "autoSync", `${path}.argo`);

  const nfs = requireObject(root.nfs, `${path}.nfs`);
  requireNonEmptyString(nfs, "server", `${path}.nfs`);
  if (nfs.zone !== undefined) {
    requireNonEmptyString(nfs, "zone", `${path}.nfs`);
  }

  const pools = requireArray(root.loadBalancerPools, `${path}.loadBalancerPools`);
  pools.forEach((entry, index) => {
    const pool = requireObject(entry, `${path}.loadBalancerPools[${index}]`);
    requireNonEmptyString(pool, "name", `${path}.loadBalancerPools[${index}]`);
    requireCidr(pool, "cidr", `${path}.loadBalancerPools[${index}]`);
  });

  // Top-level exhaustiveness guard: adding a key to ClusterConfig without
  // validating it above produces a compile error here.
  const exhaustive: Record<keyof ClusterConfig, true> = {
    id: true,
    apexDomain: true,
    aws: true,
    onePassword: true,
    kubernetes: true,
    network: true,
    dns: true,
    argo: true,
    nfs: true,
    loadBalancerPools: true,
  };
  void exhaustive;
}

function requireObject(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`${path}: must be an object`);
  }
  return value as Record<string, unknown>;
}

function requireArray(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new Error(`${path}: must be an array`);
  }
  return value;
}

function requireNonEmptyString(obj: Record<string, unknown>, key: string, path: string): void {
  const value = obj[key];
  if (typeof value !== "string") {
    throw new Error(`${path}.${key}: must be a string`);
  }
  if (value.trim().length === 0) {
    throw new Error(`${path}.${key}: must not be empty`);
  }
}

function requireBoolean(obj: Record<string, unknown>, key: string, path: string): void {
  if (typeof obj[key] !== "boolean") {
    throw new Error(`${path}.${key}: must be a boolean`);
  }
}

function requireInteger(obj: Record<string, unknown>, key: string, path: string, min: number, max: number): void {
  const value = obj[key];
  if (typeof value !== "number" || !Number.isInteger(value) || value < min || value > max) {
    throw new Error(`${path}.${key}: must be an integer from ${min} to ${max}`);
  }
}

function requireAwsAccountId(obj: Record<string, unknown>, key: string, path: string): void {
  const value = obj[key];
  if (typeof value !== "string" || !/^\d{12}$/.test(value)) {
    throw new Error(`${path}.${key}: must be a 12-digit AWS account id`);
  }
}

function requireCidr(obj: Record<string, unknown>, key: string, path: string): void {
  const value = obj[key];
  if (typeof value !== "string" || !CIDR_PATTERN.test(value)) {
    throw new Error(`${path}.${key}: must be CIDR notation (e.g. 10.42.0.0/16)`);
  }
  const [address, maskStr] = value.split("/");
  const mask = Number(maskStr);
  if (mask < 0 || mask > 32) {
    throw new Error(`${path}.${key}: CIDR mask must be 0–32`);
  }
  for (const octet of address.split(".")) {
    const n = Number(octet);
    if (n < 0 || n > 255) {
      throw new Error(`${path}.${key}: octet out of range in ${value}`);
    }
  }
}

function requireIpv4Address(obj: Record<string, unknown>, key: string, path: string): void {
  const value = obj[key];
  if (typeof value !== "string") {
    throw new Error(`${path}.${key}: must be an IPv4 address`);
  }
  const parts = value.split(".");
  if (parts.length !== 4) {
    throw new Error(`${path}.${key}: must be an IPv4 address`);
  }
  for (const part of parts) {
    if (!/^\d+$/.test(part)) {
      throw new Error(`${path}.${key}: must be an IPv4 address`);
    }
    const n = Number(part);
    if (n < 0 || n > 255) {
      throw new Error(`${path}.${key}: octet out of range in ${value}`);
    }
  }
}

function requireHttpsUrl(obj: Record<string, unknown>, key: string, path: string): void {
  const value = obj[key];
  if (typeof value !== "string") {
    throw new Error(`${path}.${key}: must be a string`);
  }
  try {
    const url = new URL(value);
    if (url.protocol !== "https:") {
      throw new Error(`${path}.${key}: must be an https:// URL`);
    }
  } catch {
    throw new Error(`${path}.${key}: must be a valid URL`);
  }
}

function requireConst(obj: Record<string, unknown>, key: string, value: string, path: string): void {
  if (obj[key] !== value) {
    throw new Error(`${path}.${key}: must equal ${JSON.stringify(value)}`);
  }
}

export type * from "./config.js";
export * from "./context.js";
