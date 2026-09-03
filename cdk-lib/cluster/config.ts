export interface ClusterConfig {
  readonly id: "v3";
  readonly apexDomain: string;
  readonly aws?: AwsConfig;
  readonly onePassword: OnePasswordConfig;
  readonly kubernetes: KubernetesConfig;
  readonly network: NetworkConfig;
  readonly dns: DnsConfig;
  readonly argo: ArgoConfig;
  readonly nfs: NfsConfig;
  readonly loadBalancerPools: LoadBalancerPoolConfig[];
}

export interface AwsConfig {
  readonly accountId: string;
  readonly region: string;
  readonly oidcIssuer?: AwsOidcIssuerConfig;
}

export interface AwsOidcIssuerConfig {
  readonly url: string;
  readonly jwksUri: string;
}

export interface OnePasswordConfig {
  readonly vault: string;
}

export interface KubernetesConfig {
  readonly api: KubernetesApiConfig;
  readonly dns: string;
  readonly domain: string;
  readonly subnets: KubernetesSubnetsConfig;
}

export interface KubernetesApiConfig {
  readonly primary: string;
  readonly vips: KubernetesApiVipConfig[];
}

export interface KubernetesApiVipConfig {
  readonly name: string;
  readonly address: string;
  readonly interface?: string;
}

export function primaryKubernetesApiVip(api: KubernetesApiConfig): KubernetesApiVipConfig {
  const primary = api.vips.find(vip => vip.name === api.primary);
  if (primary === undefined) {
    throw new Error(`No Kubernetes API VIP named ${JSON.stringify(api.primary)}`);
  }
  return primary;
}

export interface KubernetesSubnetsConfig {
  readonly pods: string;
  readonly services: string;
}

export interface NetworkConfig {
  readonly vlans: VlanConfig[];
}

export interface VlanConfig {
  readonly name: string;
  readonly id: number;
  readonly cidr: string;
}

export interface DnsConfig {
  readonly k8sGatewayServiceIp: string;
  readonly staticRecords: DnsStaticRecordConfig[];
}

export interface DnsStaticRecordConfig {
  readonly name: string;
  readonly address: string;
}

export interface ArgoConfig {
  readonly namespace: string;
  readonly project: string;
  readonly repoUrl: string;
  readonly repoBranch: string;
  readonly autoSync: boolean;
}

export interface NfsConfig {
  readonly server: string;
  readonly zone?: string;
}

export interface LoadBalancerPoolConfig {
  readonly name: string;
  readonly cidr: string;
}
