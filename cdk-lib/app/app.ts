import { ApiObject, App, JsonPatch, type AppProps, YamlOutputType } from "cdk8s";

import { contextOption, type AppOption, type Context, type ContextClass } from "../context/base.js";

export interface K2AppProps extends Omit<AppProps, "yamlOutputType"> {
  readonly options?: AppOption[];
}

export class K2App extends App {
  private readonly namespacePatchApplied = new WeakSet<ApiObject>();

  public constructor(props: K2AppProps = {}) {
    const { options, ...appProps } = props;
    super({
      yamlOutputType: YamlOutputType.FILE_PER_APP,
      ...appProps,
    });

    for (const option of options ?? []) {
      option(this);
    }
  }

  public use(option: AppOption): this;
  public use<T extends Context, Args extends unknown[]>(Ctor: ContextClass<T, Args>, ...args: Args): this;
  public use<T extends Context, Args extends unknown[]>(
    optionOrCtor: AppOption | ContextClass<T, Args>,
    ...args: Args
  ): this {
    if ("contextKey" in optionOrCtor) {
      contextOption(optionOrCtor, ...args)(this);
    } else {
      optionOrCtor(this);
    }
    return this;
  }

  public override synth(): void {
    this.removeClusterScopedNamespaces();
    super.synth();
  }

  public override synthYaml(): string {
    this.removeClusterScopedNamespaces();
    return super.synthYaml();
  }

  private removeClusterScopedNamespaces(): void {
    for (const construct of this.node.findAll()) {
      if (!ApiObject.isApiObject(construct) || this.namespacePatchApplied.has(construct)) {
        continue;
      }
      if (isClusterScopedApiObject(construct) && construct.metadata.namespace !== undefined) {
        construct.addJsonPatch(JsonPatch.remove("/metadata/namespace"));
        this.namespacePatchApplied.add(construct);
      }
    }
  }
}

function isClusterScopedApiObject(resource: ApiObject): boolean {
  return CLUSTER_SCOPED_KINDS.has(`${resource.apiVersion}/${resource.kind}`);
}

const CLUSTER_SCOPED_KINDS = new Set([
  "v1/Namespace",
  "v1/Node",
  "v1/PersistentVolume",
  "admissionregistration.k8s.io/v1/MutatingWebhookConfiguration",
  "admissionregistration.k8s.io/v1/ValidatingWebhookConfiguration",
  "apiextensions.k8s.io/v1/CustomResourceDefinition",
  "apiregistration.k8s.io/v1/APIService",
  "cert-manager.io/v1/ClusterIssuer",
  "cilium.io/v2/CiliumCIDRGroup",
  "cilium.io/v2/CiliumClusterwideNetworkPolicy",
  "cilium.io/v2/CiliumL2AnnouncementPolicy",
  "cilium.io/v2/CiliumLoadBalancerIPPool",
  "external-secrets.io/v1/ClusterExternalSecret",
  "external-secrets.io/v1/ClusterSecretStore",
  "external-secrets.io/v1alpha1/ClusterExternalSecret",
  "external-secrets.io/v1alpha1/ClusterSecretStore",
  "gateway.networking.k8s.io/v1/GatewayClass",
  "ingress.pomerium.io/v1/Pomerium",
  "networking.k8s.io/v1/IngressClass",
  "rbac.authorization.k8s.io/v1/ClusterRole",
  "rbac.authorization.k8s.io/v1/ClusterRoleBinding",
  "scheduling.k8s.io/v1/PriorityClass",
  "storage.k8s.io/v1/CSIDriver",
  "storage.k8s.io/v1/CSINode",
  "storage.k8s.io/v1/StorageClass",
  "storage.k8s.io/v1/VolumeAttachment",
]);
