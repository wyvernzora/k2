import { join, dirname } from "path";
import { fileURLToPath } from "url";

import { Construct } from "constructs";
import { ConfigMap, Deployment, DeploymentStrategy, Secret, VolumeMount, Volume } from "cdk8s-plus-32";

import { K2Volumes, oci } from "@k2/cdk-lib";

const PLEX_ROOT = "/config/Library/Application Support/Plex Media Server";
const __dirname = dirname(fileURLToPath(import.meta.url));

export interface PlexDeploymentProps {
  readonly volumes: K2Volumes<"config" | "series" | "features" | "airing">;
}
type Props = PlexDeploymentProps;

export class PlexDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    this.addPlexContainer(props);
    this.addNginxContainer();
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
      image: oci`plexinc/pms-docker:1.42.1.10060-4e8b05daf`,
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

  private addNginxContainer(): void {
    const certSecret = Secret.fromSecretName(this, "plex-cert", "default-certificate");
    const config = new ConfigMap(this, "nginx-conf");
    config.addFile(join(__dirname, "./config/nginx.conf"));

    this.addContainer({
      name: "nginx",
      image: "nginx:latest",
      ports: [
        {
          name: "http",
          number: 80,
        },
        {
          name: "https",
          number: 443,
        },
      ],
      volumeMounts: [
        {
          volume: Volume.fromSecret(this, "plex-cert-vol", certSecret),
          path: "/etc/nginx/ssl",
        },
        {
          volume: Volume.fromConfigMap(this, "plex-conf-vol", config),
          path: "/etc/nginx/conf.d",
        },
      ],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
    });
  }
}
