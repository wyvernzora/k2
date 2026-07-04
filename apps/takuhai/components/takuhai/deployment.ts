import { Duration, Size } from "cdk8s";
import {
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  type ContainerProps,
  type ISecret,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment } from "@k2/cdk-lib";

import {
  TAKUHAI_CRAWLER_DMHY_LABELS,
  TAKUHAI_CRAWLER_NYAA_LABELS,
  TAKUHAI_CRAWLER_PORT,
  TAKUHAI_HTTP_PORT,
  TAKUHAI_LABELS,
} from "../../constants.js";

const TAKUHAI_IMAGE = "ghcr.io/wyvernzora/takuhai:dev";
const TAKUHAI_CRAWLER_DMHY_IMAGE = "ghcr.io/wyvernzora/takuhai/crawler-dmhy:dev";
const TAKUHAI_CRAWLER_NYAA_IMAGE = "ghcr.io/wyvernzora/takuhai/crawler-nyaa:dev";
const APP_UID = 65532;
const APP_GID = 65532;

export interface TakuhaiDeploymentProps {
  readonly credentialsSecretName: string;
}

export class TakuhaiDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: TakuhaiDeploymentProps) {
    super(scope, id, {
      metadata: { name: "takuhai" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: TAKUHAI_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: TAKUHAI_LABELS }));
    const credentials = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    this.addContainer(takuhaiContainer(credentials));
  }
}

export class TakuhaiCrawlerDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "crawler-dmhy" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: TAKUHAI_CRAWLER_DMHY_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: TAKUHAI_CRAWLER_DMHY_LABELS }));
    this.addContainer(
      crawlerContainer({
        name: "crawler-dmhy",
        image: TAKUHAI_CRAWLER_DMHY_IMAGE,
        envVariables: {
          TAKUHAI_DMHY_ADDR: EnvValue.fromValue(`:${TAKUHAI_CRAWLER_PORT}`),
          TAKUHAI_DMHY_BASE_URL: EnvValue.fromValue("https://share.dmhy.org"),
          TZ: EnvValue.fromValue("America/Los_Angeles"),
        },
      }),
    );
  }
}

export class TakuhaiNyaaCrawlerDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: { name: "crawler-nyaa" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: TAKUHAI_CRAWLER_NYAA_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: TAKUHAI_CRAWLER_NYAA_LABELS }));
    this.addContainer(
      crawlerContainer({
        name: "crawler-nyaa",
        image: TAKUHAI_CRAWLER_NYAA_IMAGE,
        envVariables: {
          TAKUHAI_NYAA_ADDR: EnvValue.fromValue(`:${TAKUHAI_CRAWLER_PORT}`),
          TAKUHAI_NYAA_BASE_URL: EnvValue.fromValue("https://nyaa.si"),
          TAKUHAI_NYAA_CATEGORY: EnvValue.fromValue("1_4"),
          TAKUHAI_NYAA_FILTER: EnvValue.fromValue("0"),
          TZ: EnvValue.fromValue("America/Los_Angeles"),
        },
      }),
    );
  }
}

function takuhaiContainer(credentials: ISecret): ContainerProps {
  const health = Probe.fromHttpGet("/healthz", {
    port: TAKUHAI_HTTP_PORT,
    failureThreshold: 6,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
  return {
    name: "takuhai",
    image: TAKUHAI_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    ports: [{ name: "http", number: TAKUHAI_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      TAKUHAI_TRANSPORT: EnvValue.fromValue("http"),
      TAKUHAI_ADDR: EnvValue.fromValue(`:${TAKUHAI_HTTP_PORT}`),
      TAKUHAI_DATABASE_URL: credentials.envValue("uri"),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
    },
    liveness: health,
    readiness: health,
    startup: Probe.fromHttpGet("/healthz", {
      port: TAKUHAI_HTTP_PORT,
      failureThreshold: 30,
      periodSeconds: Duration.seconds(10),
      timeoutSeconds: Duration.seconds(5),
    }),
    resources: {
      cpu: {
        request: Cpu.millis(50),
        limit: Cpu.millis(1000),
      },
      memory: {
        request: Size.mebibytes(128),
        limit: Size.gibibytes(1),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: APP_UID,
      group: APP_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}

interface CrawlerContainerProps {
  readonly name: string;
  readonly image: string;
  readonly envVariables: Record<string, EnvValue>;
}

function crawlerContainer(props: CrawlerContainerProps): ContainerProps {
  const probe = Probe.fromTcpSocket({
    port: TAKUHAI_CRAWLER_PORT,
    failureThreshold: 6,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
  return {
    name: props.name,
    image: props.image,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    ports: [{ name: "http", number: TAKUHAI_CRAWLER_PORT, protocol: Protocol.TCP }],
    envVariables: props.envVariables,
    liveness: probe,
    readiness: probe,
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(512),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: APP_UID,
      group: APP_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}
