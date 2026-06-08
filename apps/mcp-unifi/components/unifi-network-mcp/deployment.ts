import { Duration, Size } from "cdk8s";
import {
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  type ContainerProps,
  type ISecret,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment } from "@k2/cdk-lib";

import { UNIFI_NETWORK_MCP_LABELS, UNIFI_NETWORK_MCP_PORT } from "../../constants.js";

const UNIFI_NETWORK_MCP_IMAGE = "ghcr.io/enuno/unifi-mcp-server:latest";
const MCP_UID = 1000;
const MCP_GID = 1000;

export interface UnifiNetworkMcpDeploymentProps {
  readonly credentialsSecretName: string;
  readonly host: string;
}

export class UnifiNetworkMcpDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: UnifiNetworkMcpDeploymentProps) {
    super(scope, id, {
      metadata: { name: "unifi-network-mcp" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: UNIFI_NETWORK_MCP_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: UNIFI_NETWORK_MCP_LABELS }));
    const credentials = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    this.addContainer(unifiNetworkMcpContainer(props.host, credentials));
  }
}

function unifiNetworkMcpContainer(host: string, credentials: ISecret): ContainerProps {
  const probe = Probe.fromTcpSocket({
    port: UNIFI_NETWORK_MCP_PORT,
    failureThreshold: 6,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
  return {
    name: "unifi-network-mcp",
    image: UNIFI_NETWORK_MCP_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "mcp", number: UNIFI_NETWORK_MCP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      UNIFI_API_KEY: credentials.envValue("api-key"),
      UNIFI_API_TYPE: EnvValue.fromValue("local"),
      UNIFI_LOCAL_HOST: EnvValue.fromValue(host),
      UNIFI_LOCAL_VERIFY_SSL: EnvValue.fromValue("false"),
      UNIFI_PROFILE: EnvValue.fromValue("network"),
      MCP_SERVER_TRANSPORT: EnvValue.fromValue("http"),
      MCP_SERVER_HOST: EnvValue.fromValue("0.0.0.0"),
      MCP_SERVER_PORT: EnvValue.fromValue(String(UNIFI_NETWORK_MCP_PORT)),
    },
    liveness: probe,
    readiness: probe,
    resources: {
      cpu: {
        request: Cpu.millis(50),
        limit: Cpu.millis(1000),
      },
      memory: {
        request: Size.mebibytes(128),
        limit: Size.gibibytes(1),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: MCP_UID,
      group: MCP_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: false,
    },
  };
}
