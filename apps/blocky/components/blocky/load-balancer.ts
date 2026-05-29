import type { LoadBalancerPoolConfig } from "@k2/cdk-lib";

export function blockyLoadBalancerIp(pools: LoadBalancerPoolConfig[]): string {
  const pool = pools.find(candidate => candidate.name === "blocky");
  if (pool === undefined) {
    throw new Error("Blocky requires a loadBalancerPools entry named blocky");
  }

  const [address, mask] = pool.cidr.split("/");
  if (mask !== "32") {
    throw new Error("Blocky loadBalancerPools.blocky must be a single-IP /32 CIDR");
  }
  return address;
}
