import type { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

import {
  DEFAULT_CERTIFICATE_REPLICATION_STORE_NAME,
  DEFAULT_CERTIFICATE_SECRET_NAME,
} from "../cert-manager/constants.js";

const TARGET_NAMESPACES = ["plex", "traefik"];
const {
  ClusterExternalSecret,
  ClusterExternalSecretSpecExternalSecretSpecDataRemoteRefConversionStrategy: RemoteRefConversionStrategy,
  ClusterExternalSecretSpecExternalSecretSpecDataRemoteRefDecodingStrategy: RemoteRefDecodingStrategy,
  ClusterExternalSecretSpecExternalSecretSpecDataRemoteRefMetadataPolicy: RemoteRefMetadataPolicy,
  ClusterExternalSecretSpecExternalSecretSpecDataRemoteRefNullBytePolicy: RemoteRefNullBytePolicy,
  ClusterExternalSecretSpecExternalSecretSpecSecretStoreRefKind: SecretStoreRefKind,
  ClusterExternalSecretSpecExternalSecretSpecTargetCreationPolicy: TargetCreationPolicy,
  ClusterExternalSecretSpecExternalSecretSpecTargetDeletionPolicy: TargetDeletionPolicy,
  ClusterExternalSecretSpecExternalSecretSpecTargetTemplateEngineVersion: TargetTemplateEngineVersion,
  ClusterExternalSecretSpecExternalSecretSpecTargetTemplateMergePolicy: TargetTemplateMergePolicy,
} = crd;

export class DefaultCertificateClusterExternalSecret extends ClusterExternalSecret {
  public constructor(scope: Construct, id: string) {
    super(scope, id, clusterExternalSecretProps());
  }
}

function clusterExternalSecretProps() {
  return {
    metadata: { name: DEFAULT_CERTIFICATE_SECRET_NAME },
    spec: clusterExternalSecretSpec(),
  };
}

function clusterExternalSecretSpec() {
  return {
    externalSecretName: DEFAULT_CERTIFICATE_SECRET_NAME,
    namespaceSelectors: TARGET_NAMESPACES.map(namespaceSelector),
    refreshTime: "1m",
    externalSecretSpec: externalSecretSpec(),
  };
}

function namespaceSelector(name: string) {
  return {
    matchLabels: {
      "kubernetes.io/metadata.name": name,
    },
  };
}

function externalSecretSpec() {
  return {
    refreshInterval: "1h",
    secretStoreRef: secretStoreRef(),
    target: targetSecret(),
    data: [tlsData("tls.crt"), tlsData("tls.key")],
  };
}

function secretStoreRef() {
  return {
    kind: SecretStoreRefKind.CLUSTER_SECRET_STORE,
    name: DEFAULT_CERTIFICATE_REPLICATION_STORE_NAME,
  };
}

function targetSecret() {
  return {
    name: DEFAULT_CERTIFICATE_SECRET_NAME,
    creationPolicy: TargetCreationPolicy.OWNER,
    deletionPolicy: TargetDeletionPolicy.RETAIN,
    template: targetTemplate(),
  };
}

function targetTemplate() {
  return {
    engineVersion: TargetTemplateEngineVersion.V2,
    mergePolicy: TargetTemplateMergePolicy.REPLACE,
    type: "kubernetes.io/tls",
  };
}

function tlsData(key: string) {
  return {
    secretKey: key,
    remoteRef: {
      key: DEFAULT_CERTIFICATE_SECRET_NAME,
      property: key,
      conversionStrategy: RemoteRefConversionStrategy.DEFAULT,
      decodingStrategy: RemoteRefDecodingStrategy.NONE,
      metadataPolicy: RemoteRefMetadataPolicy.NONE,
      nullBytePolicy: RemoteRefNullBytePolicy.IGNORE,
    },
  };
}
