export interface ClusterConfig {
  readonly id: "v3";
  readonly apexDomain: string;
  readonly aws?: AwsConfig;
  readonly onePassword: OnePasswordConfig;
  readonly kubernetes: KubernetesConfig;
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
  readonly api: string;
  readonly dns: string;
  readonly domain: string;
  readonly subnets: KubernetesSubnetsConfig;
}

export interface KubernetesSubnetsConfig {
  readonly pods: string;
  readonly services: string;
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
