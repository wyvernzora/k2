import { Cpu, Deployment, DeploymentStrategy, EnvValue, Probe } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { K2Volumes, oci } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";
import {} from "cdk8s-plus-28/lib/imports/k8s";
import { Size } from "cdk8s";

export interface N8NDeploymentProps {
  readonly volumes: K2Volumes<"appdata">;
}
type Props = N8NDeploymentProps;

export class N8NDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      securityContext: {
        fsGroup: 1000,
        user: 1000,
      },
      strategy: DeploymentStrategy.recreate(),
    });
    this.addN8NContainer(props);
    this.addLunchMoneyMCPContainer();
  }

  private addN8NContainer(props: Props): void {
    this.addContainer({
      name: "n8n",
      image: oci`n8nio/n8n:1.104.1`,
      ports: [
        {
          name: "http",
          number: 5678,
        },
      ],
      volumeMounts: [props.volumes.appdata(this, "vol-appdata").mount(this, { path: "/home/node/.n8n" })],
      envVariables: {
        N8N_PORT: { value: "5678" },
        N8N_USER_MANAGEMENT_ENABLED: { value: "false" },
        GENERIC_TIMEZONE: { value: "America/Los_Angeles" },
        TZ: { value: "America/Los_Angeles" },
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
          request: Size.gibibytes(0.5),
          limit: Size.gibibytes(4),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(10),
        },
      },
      liveness: Probe.fromHttpGet("/", { port: 5678 }),
      readiness: Probe.fromHttpGet("/", { port: 5678 }),
    });
  }

  private addLunchMoneyMCPContainer(): void {
    const token = new K2Secret(this, "lm-token", {
      itemId: "3hzvddfjcii34wz2ej6g4zbwf4",
    });
    this.addContainer({
      name: "lunchmoney-mcp",
      image: oci`ghcr.io/wyvernzora/lunchmoney-mcp-server:dev`,
      ports: [
        {
          name: "mcp",
          number: 3000,
        },
      ],
      envVariables: {
        LUNCHMONEY_TOKEN: EnvValue.fromSecretValue({
          secret: token.secret,
          key: "credential",
        }),
      },
    });
  }
}
