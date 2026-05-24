import type { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

import { DEFAULT_CERTIFICATE_REPLICATION_STORE_NAME } from "../cert-manager/constants.js";

const { ClusterSecretStore, ClusterSecretStoreSpecProviderKubernetesServerCaProviderType: CaProviderType } = crd;

export interface DefaultCertificateReplicationStoreProps {
  readonly serviceAccountName: string;
  readonly sourceNamespace: string;
}

export class DefaultCertificateReplicationStore extends ClusterSecretStore {
  public constructor(scope: Construct, id: string, props: DefaultCertificateReplicationStoreProps) {
    super(scope, id, replicationStoreProps(props));
  }
}

function replicationStoreProps(props: DefaultCertificateReplicationStoreProps) {
  return {
    metadata: { name: DEFAULT_CERTIFICATE_REPLICATION_STORE_NAME },
    spec: replicationStoreSpec(props),
  };
}

function replicationStoreSpec(props: DefaultCertificateReplicationStoreProps) {
  return {
    provider: {
      kubernetes: kubernetesProvider(props),
    },
  };
}

function kubernetesProvider(props: DefaultCertificateReplicationStoreProps) {
  return {
    remoteNamespace: props.sourceNamespace,
    server: kubernetesServer(props.sourceNamespace),
    auth: serviceAccountAuth(props),
  };
}

function kubernetesServer(sourceNamespace: string) {
  return {
    caProvider: {
      type: CaProviderType.CONFIG_MAP,
      name: "kube-root-ca.crt",
      namespace: sourceNamespace,
      key: "ca.crt",
    },
  };
}

function serviceAccountAuth(props: DefaultCertificateReplicationStoreProps) {
  return {
    serviceAccount: {
      name: props.serviceAccountName,
      namespace: props.sourceNamespace,
    },
  };
}
