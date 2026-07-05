import { Size } from "cdk8s";
import {
  Cpu,
  DeploymentStrategy,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  type ContainerProps,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment, oci } from "@k2/cdk-lib";

import { DMHY_MCP_LABELS, DMHY_MCP_PORT } from "../../constants.js";

const DMHY_MCP_IMAGE = oci`ghcr.io/wyvernzora/dmhy-mcp:dev`;
const NOBODY_UID = 65532;
const NOBODY_GID = 65532;

export class DmhyMcpDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "dmhy-mcp" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: DMHY_MCP_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: DMHY_MCP_LABELS }));
    this.addContainer(dmhyMcpContainer());
  }
}

function dmhyMcpContainer(): ContainerProps {
  const probe = Probe.fromHttpGet("/healthz", { port: DMHY_MCP_PORT });
  return {
    name: "dmhy-mcp",
    image: DMHY_MCP_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    ports: [{ name: "mcp", number: DMHY_MCP_PORT, protocol: Protocol.TCP }],
    liveness: probe,
    readiness: probe,
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(32),
        limit: Size.mebibytes(256),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: NOBODY_UID,
      group: NOBODY_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}
