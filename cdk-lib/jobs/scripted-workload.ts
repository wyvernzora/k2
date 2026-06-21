import { Duration, Size } from "cdk8s";
import {
  Capability,
  ConfigMap,
  Cpu,
  ImagePullPolicy,
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
import type { Construct } from "constructs";

import { Namespace } from "../context/namespace.js";

const JOB_RUNNER_IMAGE = "ghcr.io/wyvernzora/k2-job-runner:v2";
const SCRIPTED_WORKLOAD_TYPE_LABEL = "k2.wyvernzora.io/scripted-workload-type";
const SCRIPT_MOUNT_PATH = "/scripts";
const SCRIPT_VOLUME_NAME = "script";
const TMP_VOLUME_NAME = "tmp";

export type ScriptedWorkloadType = "job" | "cronjob";

export interface ScriptedWorkloadScript {
  readonly path: string;
  readonly filename: string;
}

export interface ScriptedWorkloadRbacRule {
  readonly apiGroups: string[];
  readonly resources: string[];
  readonly verbs: string[];
  readonly resourceNames?: string[];
}

export interface ScriptedWorkloadMount {
  readonly volume: Volume;
  readonly path: string;
  readonly readOnly?: boolean;
  readonly subPath?: string;
  readonly subPathExpr?: string;
  readonly propagation?: VolumeMount["propagation"];
}

export interface ScriptedWorkloadProps {
  readonly name: string;
  readonly script: ScriptedWorkloadScript;
  readonly image?: string;
  readonly imagePullPolicy?: ImagePullPolicy;
  readonly containerName?: string;
  readonly command?: string[];
  readonly env?: Record<string, EnvValue>;
  readonly labels?: Record<string, string>;
  readonly rbacRules?: ScriptedWorkloadRbacRule[];
  readonly mounts?: ScriptedWorkloadMount[];
  readonly resources?: ContainerProps["resources"];
  readonly securityContext?: ContainerProps["securityContext"];
}

export interface PreparedScriptedWorkload {
  readonly serviceAccount?: IServiceAccount;
  readonly jobProps: JobProps;
}

export interface PrepareScriptedWorkloadOptions {
  readonly type: ScriptedWorkloadType;
}

export function prepareScriptedWorkload(
  scope: Construct,
  props: ScriptedWorkloadProps,
  options: PrepareScriptedWorkloadOptions,
): PreparedScriptedWorkload {
  const script = new ConfigMap(scope, "script", {
    metadata: { name: `${props.name}-script` },
  });
  script.addFile(props.script.path, props.script.filename);

  const scriptVolume = Volume.fromConfigMap(scope, "script-volume", script, {
    name: SCRIPT_VOLUME_NAME,
    defaultMode: 365,
  });
  const tmpVolume = Volume.fromEmptyDir(scope, "tmp-volume", TMP_VOLUME_NAME, { sizeLimit: Size.mebibytes(16) });
  const serviceAccount = createServiceAccount(scope, props);

  return {
    serviceAccount,
    jobProps: scriptedJobProps({
      props,
      scriptVolume,
      tmpVolume,
      serviceAccount,
      type: options.type,
    }),
  };
}

function createServiceAccount(scope: Construct, props: ScriptedWorkloadProps): IServiceAccount | undefined {
  const rbacRules = props.rbacRules ?? [];
  if (rbacRules.length === 0) {
    return undefined;
  }

  const serviceAccount = new ServiceAccount(scope, "service-account", {
    metadata: { name: props.name },
  });
  createRbac(scope, props.name, rbacRules, serviceAccount);
  return serviceAccount;
}

function createRbac(
  scope: Construct,
  name: string,
  rules: ScriptedWorkloadRbacRule[],
  serviceAccount: IServiceAccount,
): void {
  const namespace = Namespace.of(scope).namespace;

  // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Role L2 does not expose resourceNames.
  new k8s.KubeRole(scope, "role", {
    metadata: { name },
    rules,
  });
  // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- Keep the raw RoleBinding paired with the resourceNames-capable Role above.
  new k8s.KubeRoleBinding(scope, "role-binding", {
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

interface ScriptedJobPropsOptions {
  readonly props: ScriptedWorkloadProps;
  readonly serviceAccount?: IServiceAccount;
  readonly scriptVolume: Volume;
  readonly tmpVolume: Volume;
  readonly type: ScriptedWorkloadType;
}

function scriptedJobProps(options: ScriptedJobPropsOptions): JobProps {
  return {
    metadata: { name: options.props.name },
    backoffLimit: 6,
    restartPolicy: RestartPolicy.ON_FAILURE,
    ttlAfterFinished: Duration.days(1),
    podMetadata: { labels: podLabels(options.props.labels, options.type) },
    automountServiceAccountToken: options.serviceAccount !== undefined,
    enableServiceLinks: false,
    securityContext: {
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
    },
    serviceAccount: options.serviceAccount,
    containers: [scriptContainer(options)],
    volumes: [options.scriptVolume, options.tmpVolume, ...extraVolumes(options.props)],
  };
}

function scriptContainer(options: ScriptedJobPropsOptions): ContainerProps {
  return {
    name: options.props.containerName ?? "script",
    image: options.props.image ?? JOB_RUNNER_IMAGE,
    imagePullPolicy: options.props.imagePullPolicy ?? ImagePullPolicy.IF_NOT_PRESENT,
    command: loggedCommand(options),
    envVariables: options.props.env,
    resources: options.props.resources ?? {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(250),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(256),
      },
    },
    securityContext: options.props.securityContext ?? {
      allowPrivilegeEscalation: false,
      capabilities: { drop: [Capability.ALL] },
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
    volumeMounts: [
      {
        volume: options.scriptVolume,
        path: SCRIPT_MOUNT_PATH,
        readOnly: true,
      },
      {
        volume: options.tmpVolume,
        path: "/tmp",
      },
      ...extraVolumeMounts(options.props),
    ],
  };
}

function loggedCommand(options: ScriptedJobPropsOptions): string[] {
  return [
    "sh",
    "-c",
    loggedCommandScript(options),
    "scripted-workload",
    ...(options.props.command ?? [`${SCRIPT_MOUNT_PATH}/${options.props.script.filename}`]),
  ];
}

function loggedCommandScript(options: ScriptedJobPropsOptions): string {
  const workload = shellLiteral(`${options.type}/${options.props.name}`);
  const script = shellLiteral(options.props.script.filename);
  return [
    `workload=${workload}`,
    `script=${script}`,
    `log() { printf '%s %s\\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$1"; }`,
    `trap 'log "$workload interrupted by SIGTERM while running $script"; exit 143' TERM`,
    `trap 'log "$workload interrupted by SIGINT while running $script"; exit 130' INT`,
    `log "$workload started $script"`,
    `"$@"`,
    `status=$?`,
    `if [ "$status" -eq 0 ]; then`,
    `  log "$workload finished $script successfully"`,
    `else`,
    `  log "$workload finished $script with exit code $status"`,
    `fi`,
    `exit "$status"`,
  ].join("\n");
}

function shellLiteral(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

function extraVolumes(props: ScriptedWorkloadProps): Volume[] {
  const volumes = new Map<string, Volume>();
  for (const mount of props.mounts ?? []) {
    volumes.set(mount.volume.name, mount.volume);
  }
  return [...volumes.values()];
}

function extraVolumeMounts(props: ScriptedWorkloadProps): VolumeMount[] {
  return props.mounts?.map(({ volume, ...mount }) => ({ volume, ...mount })) ?? [];
}

function podLabels(labels: Record<string, string> | undefined, type: ScriptedWorkloadType): Record<string, string> {
  return {
    ...labels,
    [SCRIPTED_WORKLOAD_TYPE_LABEL]: type,
  };
}
