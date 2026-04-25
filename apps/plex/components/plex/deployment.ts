import { Construct } from "constructs";
import { DeploymentStrategy, Secret, VolumeMount } from "cdk8s-plus-32";

import { K2Deployment, K2Volumes, oci } from "@k2/cdk-lib";

const PLEX_ROOT = "/config/Library/Application Support/Plex Media Server";

export interface PlexDeploymentProps {
  readonly volumes: K2Volumes<"config" | "series" | "features" | "airing">;
}
type Props = PlexDeploymentProps;

export class PlexDeployment extends K2Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    this.addPlexContainer(props);
    this.addTLSTerminationProxy(32400, Secret.fromSecretName(this, "plex-cert", "default-certificate"));
  }

  private *createVolumeMounts(volumes: Props["volumes"]): Iterable<VolumeMount> {
    yield volumes.config(this, "vol-config").mount(this, { path: PLEX_ROOT });
    yield volumes.series(this, "vol-series").mount(this, { path: "/anime/series" });
    yield volumes.features(this, "vol-features").mount(this, { path: "/anime/features" });
    yield volumes.airing(this, "vol-airing").mount(this, { path: "/anime/airing" });
  }

  private addPlexContainer(props: Props): void {
    this.addContainer({
      name: "plex-media-server",
      image: oci`plexinc/pms-docker:1.43.1.10611-1e34174b1`,
      ports: [
        {
          name: "http-internal",
          number: 32400,
        },
        {
          name: "dlna",
          number: 1900,
        },
        {
          name: "mdns",
          number: 5353,
        },
        ...[32410, 32412, 32413, 32414].map(number => ({
          name: `gdm-${number}`,
          number,
        })),
      ],
      volumeMounts: [...this.createVolumeMounts(props.volumes)],
      envVariables: {
        PLEX_UID: { value: "3000" },
        PLEX_GID: { value: "2001" },
        TZ: { value: "America/Los_Angeles" },
        VERSION: { value: "docker" },
        ADVERTISE_IP: { value: "https://plex.wyvernzora.io" },
      },
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
    });
  }
}
