import { Size } from "cdk8s";
import { Cpu, DeploymentStrategy, EnvValue, LabelSelector, Pods, Probe, Service } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Deployment, K2Volume, oci } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { METUBE_HTTP_PORT, METUBE_LABELS } from "../constants.js";

const METUBE_IMAGE = oci`ghcr.io/alexta69/metube:2026.11.10`;

export class Metube extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const deployment = new K2Deployment(this, "deployment", {
      metadata: { name: "metube" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: METUBE_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: { user: 3000, group: 2001 },
    });
    deployment.select(LabelSelector.of({ labels: METUBE_LABELS }));
    const volumes = deployment.attachVolumes({
      downloads: K2Volume.mountNfs({ path: "/mnt/data/downloads" }),
    });
    deployment.addContainer({
      name: "metube",
      image: METUBE_IMAGE,
      ports: [{ name: "http", number: METUBE_HTTP_PORT }],
      envVariables: { UMASK: EnvValue.fromValue("0007") },
      volumeMounts: [volumes.downloads("/downloads")],
      readiness: Probe.fromHttpGet("/", { port: METUBE_HTTP_PORT }),
      liveness: Probe.fromHttpGet("/", { port: METUBE_HTTP_PORT }),
      resources: {
        cpu: { request: Cpu.millis(100), limit: Cpu.units(2) },
        memory: { request: Size.mebibytes(256), limit: Size.gibibytes(2) },
      },
      securityContext: { readOnlyRootFilesystem: false },
    });
    new Service(this, "service", {
      metadata: { name: "metube" },
      selector: Pods.select(this, "service-pods", { labels: METUBE_LABELS }),
      ports: [{ name: "http", port: 80, targetPort: METUBE_HTTP_PORT }],
    });
    new AuthenticatedIngress(this, "ingress", {
      host: ApexDomain.of(this).subdomain("mt"),
      serviceName: "metube",
      servicePort: "http",
      allowWebsockets: true,
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
