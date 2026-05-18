export interface ClusterKubernetesConfig {
  readonly api: ClusterKubernetesApiConfig;
  readonly networking: ClusterKubernetesNetworkingConfig;
}

export interface ClusterKubernetesApiConfig {
  readonly vip: string;
  readonly dnsName: string;
  readonly port: number;
  readonly tlsSans: readonly string[];
}

export interface ClusterKubernetesNetworkingConfig {
  readonly podCidr: string;
  readonly serviceCidr: string;
  readonly clusterDns: string;
  readonly clusterDomain: string;
}
