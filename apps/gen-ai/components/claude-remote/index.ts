import { Chart } from "cdk8s";
import { Construct } from "constructs";

import { Namespace, VolumesOf } from "@k2/cdk-lib";

import { ClaudeRemoteConfig } from "./config.js";
import { ClaudeRemoteDeployment, ClaudeRemoteDeploymentProps } from "./deployment.js";

export interface ClaudeRemoteProps {
  readonly volumes: VolumesOf<ClaudeRemoteDeploymentProps>;
}

export class ClaudeRemote extends Chart {
  readonly config: ClaudeRemoteConfig;
  readonly deployment: ClaudeRemoteDeployment;

  constructor(scope: Construct, id: string, props: ClaudeRemoteProps) {
    super(scope, id, {
      ...Namespace.of(scope),
    });

    this.config = new ClaudeRemoteConfig(this, "config");
    this.deployment = new ClaudeRemoteDeployment(this, "depl", {
      config: this.config,
      volumes: props.volumes,
    });
  }
}
