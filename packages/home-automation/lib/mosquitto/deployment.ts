import { Construct } from "constructs";
import {
  Deployment,
  DeploymentStrategy,
  NetworkPolicy,
  NetworkPolicyTrafficDefault,
  Probe,
  Volume,
  VolumeMount,
} from "cdk8s-plus-28";
import { K2Volumes, oci } from "@k2/cdk-lib";
import { MosquittoConfig } from "./config";

export interface MosquittoDeploymentProps {
  readonly config: MosquittoConfig;
  readonly volumes: K2Volumes<"data" | "logs">;
}
type Props = MosquittoDeploymentProps;

export class MosquittoDeployment extends Deployment {
  readonly networkPolicy: NetworkPolicy;

  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    props.config.addChecksumTo(this);
    this.addMosquittoContainer(props);
    this.networkPolicy = this.createPodNetworkPolicy();
  }

  private addMosquittoContainer(props: Props): void {
    this.addContainer({
      name: "mosquitto",
      image: oci`eclipse-mosquitto:2.0.21`,
      ports: [
        {
          name: "mqtt",
          number: 1883,
        },
      ],
      volumeMounts: [...this.createVolumeMounts(props.config, props.volumes)],
      securityContext: {
        ensureNonRoot: false,
      },
      liveness: Probe.fromCommand([
        "mosquitto_sub",
        "-t",
        "$$SYS/#",
        "-C",
        "1",
        "-W",
        "5",
      ]),
      readiness: Probe.fromCommand([
        "mosquitto_sub",
        "-t",
        "$$SYS/#",
        "-C",
        "1",
        "-W",
        "5",
      ]),
    });
  }

  private *createVolumeMounts(
    config: MosquittoConfig,
    volumes: Props["volumes"],
  ): Iterable<VolumeMount> {
    yield volumes
      .data(this, "vol-data")
      .mount(this, { path: "/mosquitto/data" });
    yield volumes
      .logs(this, "vol-logs")
      .mount(this, { path: "/mosquitto/log" });
    yield {
      volume: Volume.fromConfigMap(this, "vol-conf", config),
      path: "/mosquitto/config/mosquitto.conf",
      subPath: "mosquitto.conf",
    };
  }

  private createPodNetworkPolicy(): NetworkPolicy {
    return new NetworkPolicy(this, "net-policy", {
      selector: this,
      ingress: {
        default: NetworkPolicyTrafficDefault.DENY,
      },
    });
  }
}
