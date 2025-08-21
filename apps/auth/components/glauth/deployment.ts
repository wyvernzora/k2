import { GlauthConfig } from "./config";
import { Deployment, Protocol, ISecret, Volume } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { oci } from "@k2/cdk-lib";

export interface GlauthDeploymentProps {
  readonly config: GlauthConfig;
  readonly users: ISecret;
}

export class GlauthDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: GlauthDeploymentProps) {
    super(scope, id, { replicas: 1 });
    const configVolume = Volume.fromConfigMap(this, "config", props.config);
    const usersVolume = Volume.fromSecret(this, "users", props.users);
    this.addGlauthContainer(configVolume, usersVolume);
  }

  private addGlauthContainer(config: Volume, users: Volume): void {
    this.addContainer({
      name: "glauth",
      image: oci`glauth/glauth:v2.4.0`,
      command: ["/app/glauth", "-c", "/app/conf.d/"],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      ports: [
        {
          name: "ldap",
          number: 389,
          protocol: Protocol.TCP,
        },
      ],
      volumeMounts: [
        {
          volume: config,
          path: "/app/conf.d/config.cfg",
          subPath: "config.cfg",
        },
        {
          volume: users,
          path: "/app/conf.d/users.cfg",
          subPath: "users.conf",
        },
      ],
    });
  }
}
