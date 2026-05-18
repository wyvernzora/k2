export interface ClusterCiliumConfig {
  readonly loadBalancerPool: ClusterAddressPoolConfig;
}

export interface ClusterAddressPoolConfig {
  readonly start: string;
  readonly stop: string;
}
