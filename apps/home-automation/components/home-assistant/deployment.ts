import { Deployment, DeploymentStrategy, Volume, VolumeMount } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Volumes, oci } from "@k2/cdk-lib";

import { HomeAssistantConfig } from "./config.js";

export interface HomeAssistantDeploymentProps {
  readonly config: HomeAssistantConfig;
  readonly volumes: K2Volumes<"config">;
}
type Props = HomeAssistantDeploymentProps;

export class HomeAssistantDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    const dataVolume = props.volumes.config(this, "vol-conf").mount(this, {
      path: "/config",
    });
    this.addHomeAssistantContainer(dataVolume);
    this.addInitConfigContainer(props, dataVolume);
    props.config.addChecksumTo(this);
  }

  private addHomeAssistantContainer(dataVolume: VolumeMount) {
    this.addContainer({
      name: "home-assistant",
      image: oci`linuxserver/homeassistant:2026.1.0`,
      ports: [
        {
          name: "http",
          number: 8123,
        },
      ],
      volumeMounts: [dataVolume],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
    });
  }

  private addInitConfigContainer(props: Props, dataVolume: VolumeMount) {
    this.addInitContainer({
      name: "setup-config",
      image: oci`mikefarah/yq:4`,
      command: ["/bin/sh", "-c"],
      args: ["/bin/sh /init/init.sh"],
      securityContext: {
        ensureNonRoot: false,
        user: 0,
      },
      volumeMounts: [
        dataVolume,
        {
          volume: Volume.fromConfigMap(this, "vol-init", props.config),
          path: "/init",
        },
      ],
    });
  }
}
