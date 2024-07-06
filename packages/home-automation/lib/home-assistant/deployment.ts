import { K2Volumes, oci } from "@k2/cdk-lib";
import { Deployment, DeploymentStrategy, Volume } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { HomeAssistantConfig } from "./config";

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
    this.addHomeAssistantContainer(props);
    props.config.addChecksumTo(this);
  }

  private addHomeAssistantContainer(props: Props) {
    this.addContainer({
      name: "home-assistant",
      image: oci`linuxserver/homeassistant:2024.7.1`,
      ports: [
        {
          name: "http",
          number: 8123,
        },
      ],
      volumeMounts: [
        props.volumes.config(this, "vol-conf").mount(this, {
          path: "/config",
        }),
        {
          volume: Volume.fromConfigMap(this, "vol-conf-yaml", props.config),
          path: "/config/configuration.yaml",
          subPath: "configuration.yaml",
        },
      ],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
    });
  }
}
