import { k8s, ServiceAccount } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { POMERIUM_DATABASE_SECRET_NAME, POMERIUM_DATABASE_STORAGE_SECRET_NAME } from "../../constants.js";
import * as crd from "../../../external-secrets/lib/crd.js";

const REPLICATION_SERVICE_ACCOUNT_NAME = "pomerium-database-secret-replication";
const REPLICATION_STORE_NAME = "pomerium-database-credentials";
const {
  ClusterSecretStore,
  ClusterSecretStoreSpecProviderKubernetesServerCaProviderType: CaProviderType,
  ExternalSecret,
  ExternalSecretSpecDataRemoteRefConversionStrategy: RemoteRefConversionStrategy,
  ExternalSecretSpecDataRemoteRefDecodingStrategy: RemoteRefDecodingStrategy,
  ExternalSecretSpecDataRemoteRefMetadataPolicy: RemoteRefMetadataPolicy,
  ExternalSecretSpecDataRemoteRefNullBytePolicy: RemoteRefNullBytePolicy,
  ExternalSecretSpecSecretStoreRefKind: SecretStoreRefKind,
  ExternalSecretSpecTargetCreationPolicy: TargetCreationPolicy,
  ExternalSecretSpecTargetDeletionPolicy: TargetDeletionPolicy,
  ExternalSecretSpecTargetTemplateEngineVersion: TargetTemplateEngineVersion,
  ExternalSecretSpecTargetTemplateMergePolicy: TargetTemplateMergePolicy,
} = crd;

export interface PomeriumStorageSecretProps {
  readonly namespace: string;
}

export class PomeriumStorageSecret extends Construct {
  public constructor(scope: Construct, id: string, props: PomeriumStorageSecretProps) {
    super(scope, id);

    const serviceAccount = new ServiceAccount(this, "replication-service-account", {
      metadata: { name: REPLICATION_SERVICE_ACCOUNT_NAME },
      automountToken: false,
    });
    createReplicationRbac(this, serviceAccount, props.namespace);

    new PomeriumDatabaseCredentialStore(this, "replication-store", {
      namespace: props.namespace,
      serviceAccountName: serviceAccount.name,
    });
    new PomeriumStorageExternalSecret(this, "external-secret");
  }
}

interface PomeriumDatabaseCredentialStoreProps {
  readonly namespace: string;
  readonly serviceAccountName: string;
}

class PomeriumDatabaseCredentialStore extends ClusterSecretStore {
  public constructor(scope: Construct, id: string, props: PomeriumDatabaseCredentialStoreProps) {
    super(scope, id, credentialStoreProps(props));
  }
}

class PomeriumStorageExternalSecret extends ExternalSecret {
  public constructor(scope: Construct, id: string) {
    super(scope, id, storageExternalSecretProps());
  }
}

function credentialStoreProps(props: PomeriumDatabaseCredentialStoreProps): crd.ClusterSecretStoreProps {
  return {
    metadata: { name: REPLICATION_STORE_NAME },
    spec: {
      provider: {
        kubernetes: kubernetesProvider(props),
      },
    },
  };
}

function kubernetesProvider(props: PomeriumDatabaseCredentialStoreProps): crd.ClusterSecretStoreSpecProviderKubernetes {
  return {
    remoteNamespace: props.namespace,
    server: kubernetesServer(props.namespace),
    auth: {
      serviceAccount: {
        name: props.serviceAccountName,
        namespace: props.namespace,
      },
    },
  };
}

function kubernetesServer(namespace: string): crd.ClusterSecretStoreSpecProviderKubernetesServer {
  return {
    caProvider: {
      type: CaProviderType.CONFIG_MAP,
      name: "kube-root-ca.crt",
      namespace,
      key: "ca.crt",
    },
  };
}

function storageExternalSecretProps(): crd.ExternalSecretProps {
  return {
    metadata: { name: POMERIUM_DATABASE_STORAGE_SECRET_NAME },
    spec: {
      refreshInterval: "1m",
      secretStoreRef: {
        kind: SecretStoreRefKind.CLUSTER_SECRET_STORE,
        name: REPLICATION_STORE_NAME,
      },
      target: targetSecret(),
      data: [connectionUriData()],
    },
  };
}

function targetSecret(): crd.ExternalSecretSpecTarget {
  return {
    name: POMERIUM_DATABASE_STORAGE_SECRET_NAME,
    creationPolicy: TargetCreationPolicy.OWNER,
    deletionPolicy: TargetDeletionPolicy.RETAIN,
    template: {
      engineVersion: TargetTemplateEngineVersion.V2,
      mergePolicy: TargetTemplateMergePolicy.REPLACE,
      type: "Opaque",
      data: {
        connection: "{{ .uri }}",
      },
    },
  };
}

function connectionUriData(): crd.ExternalSecretSpecData {
  return {
    secretKey: "uri",
    remoteRef: {
      key: POMERIUM_DATABASE_SECRET_NAME,
      property: "uri",
      conversionStrategy: RemoteRefConversionStrategy.DEFAULT,
      decodingStrategy: RemoteRefDecodingStrategy.NONE,
      metadataPolicy: RemoteRefMetadataPolicy.NONE,
      nullBytePolicy: RemoteRefNullBytePolicy.IGNORE,
    },
  };
}

function createReplicationRbac(scope: Construct, serviceAccount: ServiceAccount, namespace: string): void {
  // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Role L2 does not expose resourceNames.
  new k8s.KubeRole(scope, "replication-role", {
    metadata: { name: REPLICATION_SERVICE_ACCOUNT_NAME },
    rules: [
      {
        apiGroups: [""],
        resources: ["secrets"],
        resourceNames: [POMERIUM_DATABASE_SECRET_NAME],
        verbs: ["get"],
      },
      {
        apiGroups: ["authorization.k8s.io"],
        resources: ["selfsubjectrulesreviews"],
        verbs: ["create"],
      },
    ],
  });
  // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- Keep the RoleBinding paired with the resourceNames-scoped Role.
  new k8s.KubeRoleBinding(scope, "replication-role-binding", {
    metadata: { name: REPLICATION_SERVICE_ACCOUNT_NAME },
    roleRef: {
      apiGroup: "rbac.authorization.k8s.io",
      kind: "Role",
      name: REPLICATION_SERVICE_ACCOUNT_NAME,
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
