import { Size } from "cdk8s";
import {
  Cpu,
  Deployment,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  ISecret,
  Probe,
  Volume,
  VolumeMount,
} from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Volumes, oci } from "@k2/cdk-lib";

import { OpenClawConfig } from "./config.js";

export interface OpenClawDeploymentProps {
  readonly config: OpenClawConfig;
  readonly openAiSecret: ISecret;
  readonly volumes: K2Volumes<"data">;
}
type Props = OpenClawDeploymentProps;

export class OpenClawDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      securityContext: {
        fsGroup: 1000,
        user: 1000,
      },
      strategy: DeploymentStrategy.recreate(),
    });

    const dataVolume = props.volumes.data(this, "vol-data").mount(this, {
      path: "/home/node/.openclaw",
    });

    this.addInitConfigContainer(props, dataVolume);
    this.addOpenClawContainer(props, dataVolume);
    props.config.addChecksumTo(this);
  }

  private addInitConfigContainer(props: Props, dataVolume: VolumeMount): void {
    this.addInitContainer({
      name: "setup-config",
      image: oci`busybox:1.37.0`,
      command: ["/bin/sh", "-c"],
      args: [
        [
          "set -eu",
          "mkdir -p /home/node/.openclaw/workspace",
          "cp /config/openclaw.json /home/node/.openclaw/openclaw.json",
          "[ -f /home/node/.openclaw/workspace/AGENTS.md ] || cp /config/AGENTS.md /home/node/.openclaw/workspace/AGENTS.md",
        ].join("\n"),
      ],
      securityContext: {
        ensureNonRoot: false,
        user: 0,
      },
      volumeMounts: [
        dataVolume,
        {
          volume: Volume.fromConfigMap(this, "vol-config", props.config),
          path: "/config",
          readOnly: true,
        },
      ],
    });
  }

  private addOpenClawContainer(props: Props, dataVolume: VolumeMount): void {
    this.addContainer({
      name: "openclaw",
      image: oci`ghcr.io/openclaw/openclaw:2026.4.25-beta.2-slim`,
      imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
      command: ["node", "dist/index.js"],
      args: ["gateway"],
      ports: [
        {
          name: "http",
          number: 18789,
        },
      ],
      volumeMounts: [dataVolume],
      envVariables: {
        NODE_ENV: { value: "production" },
        OPENAI_API_KEY: EnvValue.fromSecretValue({
          secret: props.openAiSecret,
          key: "OPENAI_API_KEY",
        }),
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
        ephemeralStorage: {
          limit: Size.gibibytes(4),
        },
      },
      liveness: Probe.fromHttpGet("/healthz", { port: 18789 }),
      readiness: Probe.fromHttpGet("/readyz", { port: 18789 }),
    });
  }
}
