import { Duration, Size } from "cdk8s";
import {
  Capability,
  ConfigMap,
  Cpu,
  ImagePullPolicy,
  Job,
  RestartPolicy,
  ServiceAccount,
  Volume,
  k8s,
  type ContainerProps,
  type EnvValue,
  type IServiceAccount,
  type JobProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import { Construct } from "constructs";

import { Namespace } from "./context/namespace.js";
import { Scheduling } from "./scheduling.js";

const JOB_RUNNER_IMAGE = "ghcr.io/wyvernzora/k2-job-runner:v1";
const SCRIPTED_JOB_LABEL = "k2.wyvernzora.io/scripted-job";
const SCRIPT_MOUNT_PATH = "/scripts";

export interface ScriptedJobScript {
  readonly path: string;
  readonly filename: string;
}

export interface ScriptedJobRbacRule {
  readonly apiGroups: string[];
  readonly resources: string[];
  readonly verbs: string[];
  readonly resourceNames?: string[];
}

export interface ScriptedJobProps {
  readonly name: string;
  readonly script: ScriptedJobScript;
  readonly command?: string[];
  readonly env?: Record<string, EnvValue>;
  readonly labels?: Record<string, string>;
  readonly rbacRules?: ScriptedJobRbacRule[];
  readonly volumeMounts?: VolumeMount[];
}

export class ScriptedJob extends Construct {
  public readonly serviceAccount?: IServiceAccount;

  public constructor(scope: Construct, id: string, props: ScriptedJobProps) {
    super(scope, id);

    const script = new ConfigMap(this, "script", {
      metadata: { name: `${props.name}-script` },
    });
    script.addFile(props.script.path, props.script.filename);

    const rbacRules = props.rbacRules ?? [];
    if (rbacRules.length > 0) {
      this.serviceAccount = new ServiceAccount(this, "service-account", {
        metadata: { name: props.name },
      });
      this.createRbac(props.name, rbacRules, this.serviceAccount);
    }

    new ScriptedKubernetesJob(this, "job", {
      props,
      serviceAccount: this.serviceAccount,
      scriptVolume: Volume.fromConfigMap(this, "script-volume", script, {
        name: "script",
        defaultMode: 365,
      }),
      tmpVolume: Volume.fromEmptyDir(this, "tmp-volume", "tmp", { sizeLimit: Size.mebibytes(16) }),
    });
  }

  private createRbac(name: string, rules: ScriptedJobRbacRule[], serviceAccount: IServiceAccount): void {
    const namespace = Namespace.of(this).namespace;

    // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Role L2 does not expose resourceNames.
    new k8s.KubeRole(this, "role", {
      metadata: { name },
      rules,
    });
    // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- Keep the raw RoleBinding paired with the resourceNames-capable Role above.
    new k8s.KubeRoleBinding(this, "role-binding", {
      metadata: { name },
      roleRef: {
        apiGroup: "rbac.authorization.k8s.io",
        kind: "Role",
        name,
      },
      subjects: [
        {
          kind: "ServiceAccount",
          name: serviceAccount.name,
          namespace,
        },
      ],
    });
  }
}

interface ScriptedKubernetesJobProps {
  readonly props: ScriptedJobProps;
  readonly serviceAccount?: IServiceAccount;
  readonly scriptVolume: Volume;
  readonly tmpVolume: Volume;
}

class ScriptedKubernetesJob extends Job {
  public constructor(scope: Construct, id: string, props: ScriptedKubernetesJobProps) {
    super(scope, id, scriptedKubernetesJobProps(props));
    Scheduling.applyWorkersPreferred(this);
  }
}

function scriptedKubernetesJobProps(props: ScriptedKubernetesJobProps): JobProps {
  return {
    metadata: { name: props.props.name },
    backoffLimit: 6,
    restartPolicy: RestartPolicy.ON_FAILURE,
    ttlAfterFinished: Duration.days(1),
    podMetadata: { labels: podLabels(props.props.labels) },
    automountServiceAccountToken: props.serviceAccount !== undefined,
    enableServiceLinks: false,
    securityContext: {
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
    },
    serviceAccount: props.serviceAccount,
    containers: [scriptContainer(props)],
  };
}

function scriptContainer(props: ScriptedKubernetesJobProps): ContainerProps {
  return {
    name: "script",
    image: JOB_RUNNER_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: props.props.command ?? [`${SCRIPT_MOUNT_PATH}/${props.props.script.filename}`],
    envVariables: props.props.env,
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(250),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(256),
      },
    },
    securityContext: {
      allowPrivilegeEscalation: false,
      capabilities: { drop: [Capability.ALL] },
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
    volumeMounts: [
      {
        volume: props.scriptVolume,
        path: SCRIPT_MOUNT_PATH,
        readOnly: true,
      },
      {
        volume: props.tmpVolume,
        path: "/tmp",
      },
      ...(props.props.volumeMounts ?? []),
    ],
  };
}

function podLabels(labels: Record<string, string> | undefined): Record<string, string> {
  return {
    ...labels,
    [SCRIPTED_JOB_LABEL]: "true",
  };
}
