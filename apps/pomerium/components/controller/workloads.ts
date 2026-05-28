import { IntOrString, KubeDeployment, KubeJob, Quantity, type Container } from "cdk8s-plus-32/lib/imports/k8s.js";
import type { Construct } from "constructs";

import { Scheduling } from "@k2/cdk-lib";

import { POMERIUM_CONTROLLER_NAME, POMERIUM_LABELS, POMERIUM_PROXY_SERVICE_NAME } from "../../lib/constants.js";

import { metadata } from "./metadata.js";
import { GEN_SECRETS_SERVICE_ACCOUNT } from "./rbac.js";

const POMERIUM_IMAGE = "pomerium/ingress-controller:v0.32.8";
type SchedulingProfile = ReturnType<typeof Scheduling.workersPreferred>;

export function createWorkloads(scope: Construct, scheduling: SchedulingProfile): void {
  new KubeDeployment(scope, "deployment", deployment(scheduling));
  new KubeJob(scope, "gen-secrets-job", genSecretsJob(scheduling));
}

function deployment(scheduling: SchedulingProfile) {
  const deployMetadata = metadata("pomerium");
  return {
    metadata: {
      ...deployMetadata,
      labels: { ...deployMetadata.labels, ...POMERIUM_LABELS },
    },
    spec: deploymentSpec(scheduling),
  };
}

function deploymentSpec(scheduling: SchedulingProfile) {
  return {
    replicas: 1,
    selector: selector(),
    template: podTemplate(scheduling, POMERIUM_CONTROLLER_NAME, [controllerContainer()]),
  };
}

function genSecretsJob(scheduling: SchedulingProfile) {
  return {
    metadata: metadata("pomerium-gen-secrets"),
    spec: { template: jobPodTemplate(scheduling) },
  };
}

function jobPodTemplate(scheduling: SchedulingProfile) {
  return {
    metadata: { name: "pomerium-gen-secrets" },
    spec: jobPodSpec(scheduling),
  };
}

function selector() {
  return { matchLabels: { "app.kubernetes.io/name": "pomerium" } };
}

function podTemplate(scheduling: SchedulingProfile, serviceAccountName: string, containers: Container[]) {
  return {
    metadata: { labels: POMERIUM_LABELS },
    spec: podSpec(scheduling, serviceAccountName, containers),
  };
}

function podSpec(scheduling: SchedulingProfile, serviceAccountName: string, containers: Container[]) {
  return {
    affinity: scheduling.affinity,
    tolerations: scheduling.tolerations,
    containers,
    nodeSelector: { "kubernetes.io/os": "linux" },
    securityContext: { runAsNonRoot: true },
    serviceAccountName,
    terminationGracePeriodSeconds: 10,
    volumes: [{ emptyDir: {}, name: "tmp" }],
  };
}

function jobPodSpec(scheduling: SchedulingProfile) {
  return {
    affinity: scheduling.affinity,
    tolerations: scheduling.tolerations,
    containers: [genSecretsContainer()],
    nodeSelector: { "kubernetes.io/os": "linux" },
    restartPolicy: "OnFailure",
    securityContext: {
      fsGroup: 1000,
      runAsNonRoot: true,
      runAsUser: 1000,
    },
    serviceAccountName: GEN_SECRETS_SERVICE_ACCOUNT,
  };
}

function controllerContainer(): Container {
  return {
    name: "pomerium",
    image: POMERIUM_IMAGE,
    imagePullPolicy: "IfNotPresent",
    args: controllerArgs(),
    env: controllerEnv(),
    ports: controllerPorts(),
    livenessProbe: httpProbe("/healthz", 28080, 10),
    readinessProbe: httpProbe("/readyz", 28080, 5),
    startupProbe: startupProbe(),
    resources: {
      limits: { cpu: quantity("1000m"), memory: quantity("1Gi") },
      requests: { cpu: quantity("100m"), memory: quantity("200Mi") },
    },
    securityContext: controllerSecurityContext(),
    volumeMounts: [{ mountPath: "/tmp", name: "tmp" }],
  };
}

function genSecretsContainer(): Container {
  return {
    name: "gen-secrets",
    image: POMERIUM_IMAGE,
    imagePullPolicy: "IfNotPresent",
    args: ["gen-secrets", "--secrets=$(POD_NAMESPACE)/bootstrap"],
    env: [fieldEnv("POD_NAMESPACE", "metadata.namespace")],
    securityContext: { allowPrivilegeEscalation: false },
  };
}

function controllerArgs() {
  return [
    "all-in-one",
    "--pomerium-config=global",
    `--update-status-from-service=$(POMERIUM_NAMESPACE)/${POMERIUM_PROXY_SERVICE_NAME}`,
    "--metrics-bind-address=$(POD_IP):9090",
    "--health-probe-bind-address=$(POD_IP):28080",
  ];
}

function controllerEnv() {
  return [
    { name: "TMPDIR", value: "/tmp" },
    { name: "XDG_CACHE_HOME", value: "/tmp" },
    fieldEnv("POMERIUM_NAMESPACE", "metadata.namespace", "v1"),
    fieldEnv("POD_IP", "status.podIP"),
  ];
}

function controllerPorts() {
  return [
    { containerPort: 8443, name: "https", protocol: "TCP" },
    { containerPort: 443, name: "quic", protocol: "UDP" },
    { containerPort: 8080, name: "http", protocol: "TCP" },
    { containerPort: 9090, name: "metrics", protocol: "TCP" },
  ];
}

function controllerSecurityContext() {
  return {
    allowPrivilegeEscalation: false,
    capabilities: { drop: ["ALL"] },
    readOnlyRootFilesystem: true,
    runAsGroup: 65532,
    runAsNonRoot: true,
    runAsUser: 65532,
  };
}

function startupProbe() {
  return {
    failureThreshold: 40,
    httpGet: { path: "/startupz", port: portNumber(28080) },
    periodSeconds: 15,
  };
}

function fieldEnv(name: string, fieldPath: string, apiVersion?: string) {
  return {
    name,
    valueFrom: {
      fieldRef: {
        apiVersion,
        fieldPath,
      },
    },
  };
}

function httpProbe(path: string, port: number, failureThreshold: number) {
  return {
    failureThreshold,
    httpGet: { path, port: portNumber(port) },
    initialDelaySeconds: 15,
    periodSeconds: 60,
  };
}

function portNumber(port: number) {
  return IntOrString.fromNumber(port);
}

function quantity(value: string) {
  return Quantity.fromString(value);
}
