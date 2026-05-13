import { Size } from "cdk8s";
import { Cpu, Deployment, DeploymentStrategy, EnvValue, ImagePullPolicy, Probe, Volume } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Volumes, oci } from "@k2/cdk-lib";

import { ClaudeRemoteConfig } from "./config.js";

export interface ClaudeRemoteDeploymentProps {
  readonly config: ClaudeRemoteConfig;
  readonly volumes: K2Volumes<"state" | "workspace">;
}
type Props = ClaudeRemoteDeploymentProps;

export class ClaudeRemoteDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
      securityContext: {
        ensureNonRoot: true,
        user: 1001,
        group: 1001,
        fsGroup: 1001,
      },
    });

    const state = props.volumes.state(this, "vol-state").mount(this, {
      path: "/home/claude",
    });
    const workspace = props.volumes.workspace(this, "vol-workspace").mount(this, {
      path: "/workspace",
    });

    this.addContainer({
      name: "claude-remote",
      image: oci`ghcr.io/wyvernzora/claude-remote:2.1.140`,
      imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
      volumeMounts: [
        state,
        workspace,
        {
          volume: Volume.fromConfigMap(this, "vol-mcp-config", props.config),
          path: "/var/run/config/claude",
          readOnly: true,
        },
      ],
      envVariables: {
        CLAUDE_REMOTE_NAME: EnvValue.fromValue("K2 Claude Remote"),
        CLAUDE_REMOTE_CREATE_SESSION_IN_DIR: EnvValue.fromValue("false"),
      },
      securityContext: {
        ensureNonRoot: true,
        readOnlyRootFilesystem: false,
      },
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(2000),
        },
        memory: {
          request: Size.gibibytes(0.5),
          limit: Size.gibibytes(2),
        },
      },
      liveness: Probe.fromCommand(["claude-health"]),
    });

    props.config.addChecksumTo(this);
  }
}
