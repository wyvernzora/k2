import { Cpu, Deployment, Protocol, Volume } from "cdk8s-plus-28";
import { BlockyConfig } from "./config";
import { Construct } from "constructs";
import { Size } from "cdk8s";

export interface BlockyDeploymentProps {
  readonly replicas: number;
  readonly config: BlockyConfig;
}

export class BlockyDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: BlockyDeploymentProps) {
    super(scope, id, { replicas: props.replicas });
    const configVolume = Volume.fromConfigMap(this, "config", props.config);
    this.addBlockyContainer(configVolume);
  }

  private addBlockyContainer(config: Volume): void {
    this.addContainer({
      name: "blocky",
      image: "ghcr.io/0xerr0r/blocky",
      ports: [
        {
          name: "dns-udp",
          number: 53,
          protocol: Protocol.UDP,
        },
        {
          name: "http",
          number: 4000,
          protocol: Protocol.TCP,
        },
      ],
      envVariables: {
        TZ: { value: "America/Los_Angeles" },
      },
      volumeMounts: [
        {
          volume: config,
          path: "/app/config.yml",
          subPath: "blocky.yaml",
        },
      ],
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(250),
        },
        memory: {
          request: Size.mebibytes(256),
          limit: Size.mebibytes(1024),
        },
      },
    });
  }
}
