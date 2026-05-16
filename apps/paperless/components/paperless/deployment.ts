import { Cpu, Deployment, DeploymentStrategy, EnvValue, ISecret, Probe, VolumeMount } from "cdk8s-plus-32";
import { Construct } from "constructs";
import { Duration, Size } from "cdk8s";

import { K2Volumes, oci, VolumesOf } from "@k2/cdk-lib";

import { PaperlessDatabase } from "./database.js";

export interface PaperlessDeploymentProps {
  readonly database: PaperlessDatabase;
  readonly redisHost: string;
  readonly secret: ISecret;
  readonly volumes: K2Volumes<"data" | "media" | "consume" | "export">;
}
type Props = PaperlessDeploymentProps;

export class PaperlessDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
      enableServiceLinks: false,
      securityContext: {
        fsGroup: 2002,
      },
    });

    this.addMediaDirectoriesInitContainer(props);
    this.addPaperlessContainer(props);
  }

  private *createVolumeMounts(volumes: VolumesOf<Props>): Iterable<VolumeMount> {
    yield volumes.data(this, "vol-data").mount(this, { path: "/usr/src/paperless/data" });
    yield volumes.media(this, "vol-media").mount(this, { path: "/usr/src/paperless/media" });
    yield volumes.consume(this, "vol-consume").mount(this, { path: "/usr/src/paperless/consume" });
    yield volumes.export(this, "vol-export").mount(this, { path: "/usr/src/paperless/export" });
  }

  private addMediaDirectoriesInitContainer(props: Props): void {
    this.addInitContainer({
      name: "init-media-directories",
      image: oci`busybox:1.37`,
      command: ["/bin/sh", "-c"],
      args: [
        [
          "set -eu",
          "umask 0007",
          "mkdir -p /usr/src/paperless/media/documents/archive",
          "mkdir -p /usr/src/paperless/media/documents/originals",
          "mkdir -p /usr/src/paperless/media/documents/thumbnails",
          "chmod 2770 /usr/src/paperless/media/documents || true",
          "chmod 2770 /usr/src/paperless/media/documents/archive || true",
          "chmod 2770 /usr/src/paperless/media/documents/originals || true",
          "chmod 2770 /usr/src/paperless/media/documents/thumbnails || true",
        ].join("\n"),
      ],
      volumeMounts: [props.volumes.media(this, "vol-init-media").mount(this, { path: "/usr/src/paperless/media" })],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: true,
        user: 0,
      },
    });
  }

  private addPaperlessContainer(props: Props): void {
    const health = Probe.fromHttpGet("/", {
      port: 8000,
      timeoutSeconds: Duration.seconds(5),
    });
    const startup = Probe.fromHttpGet("/", {
      port: 8000,
      failureThreshold: 60,
      periodSeconds: Duration.seconds(10),
      timeoutSeconds: Duration.seconds(5),
    });

    this.addContainer({
      name: "paperless",
      image: oci`ghcr.io/paperless-ngx/paperless-ngx:2.20.15`,
      ports: [
        {
          name: "http",
          number: 8000,
        },
      ],
      volumeMounts: [...this.createVolumeMounts(props.volumes)],
      envVariables: {
        PAPERLESS_SECRET_KEY: EnvValue.fromSecretValue({
          secret: props.secret,
          key: "secretKey",
        }),
        PAPERLESS_ADMIN_USER: EnvValue.fromSecretValue({
          secret: props.secret,
          key: "adminUsername",
        }),
        PAPERLESS_ADMIN_PASSWORD: EnvValue.fromSecretValue({
          secret: props.secret,
          key: "adminPassword",
        }),
        PAPERLESS_DBHOST: EnvValue.fromSecretValue({
          secret: props.database.credentials,
          key: "host",
        }),
        PAPERLESS_DBPORT: EnvValue.fromSecretValue({
          secret: props.database.credentials,
          key: "port",
        }),
        PAPERLESS_DBNAME: EnvValue.fromSecretValue({
          secret: props.database.credentials,
          key: "dbname",
        }),
        PAPERLESS_DBUSER: EnvValue.fromSecretValue({
          secret: props.database.credentials,
          key: "user",
        }),
        PAPERLESS_DBPASS: EnvValue.fromSecretValue({
          secret: props.database.credentials,
          key: "password",
        }),
        PAPERLESS_REDIS_PASSWORD: EnvValue.fromSecretValue({
          secret: props.secret,
          key: "redisPassword",
        }),
        PAPERLESS_REDIS: {
          value: `redis://:$(PAPERLESS_REDIS_PASSWORD)@${props.redisHost}:6379`,
        },
        PAPERLESS_URL: { value: "https://paperless.wyvernzora.io" },
        PAPERLESS_TIME_ZONE: { value: "America/Los_Angeles" },
        PAPERLESS_OCR_LANGUAGE: { value: "eng" },
        PAPERLESS_CONSUMER_POLLING: { value: "60" },
        PAPERLESS_CSRF_TRUSTED_ORIGINS: { value: "https://paperless.wyvernzora.io" },
        PAPERLESS_ENABLE_HTTP_REMOTE_USER: { value: "true" },
        PAPERLESS_ENABLE_HTTP_REMOTE_USER_API: { value: "true" },
        PAPERLESS_HTTP_REMOTE_USER_HEADER_NAME: { value: "HTTP_REMOTE_USER" },
        PAPERLESS_DISABLE_REGULAR_LOGIN: { value: "true" },
        PAPERLESS_ACCOUNT_ALLOW_SIGNUPS: { value: "false" },
        USERMAP_UID: { value: "3003" },
        USERMAP_GID: { value: "2002" },
      },
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(2000),
        },
        memory: {
          request: Size.mebibytes(512),
          limit: Size.gibibytes(4),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(10),
        },
      },
      liveness: health,
      readiness: health,
      startup,
    });
  }
}
