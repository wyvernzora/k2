import { Cpu, Deployment, Protocol, Volume } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { oci } from "@k2/cdk-lib";

import { BlockyConfig } from "./config.js";

export interface BlockyDeploymentProps {
  readonly replicas: number;
  readonly config: BlockyConfig;
}

export class BlockyDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: BlockyDeploymentProps) {
    super(scope, id, { replicas: props.replicas });
    props.config.addChecksumTo(this);
    const configVolume = Volume.fromConfigMap(this, "config", props.config);
    this.addBlockyContainer(configVolume);
  }

  private addBlockyContainer(config: Volume): void {
    this.addContainer({
      name: "blocky",
      image: oci`ghcr.io/0xerr0r/blocky:v0.26`,
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
