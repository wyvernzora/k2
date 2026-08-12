import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

import {
  APPLIANCE_FABRIC_ADDRESS,
  APPLIANCE_SSH_PORT,
  DRIVER_CONFIG_SECRET_NAME,
  NODE_STAGE_SECRET_NAME,
} from "../../constants.js";

const ISCSI_PORTAL = `${APPLIANCE_FABRIC_ADDRESS}:3260`;
const IQN_BASE = "iqn.2026-07.io.wyvernzora.k2:storage";
const DATASET_PARENT = "tank/csi/k2";
const DETACHED_SNAPSHOTS_DATASET_PARENT = "tank/csi/k2-snapshots";

/**
 * 1Password item holding the appliance credentials (fields: csiPrivateKey,
 * chapUsername, chapPassword), sourced from the provisioner's
 * storage-appliance.json escrow. Item IDs are stable opaque identifiers,
 * not secrets.
 */
const CREDENTIALS_ITEM_ID = "a5lkltqnv43tdtbfn6gayexafq";

const SecretStoreKind = crd.ExternalSecretSpecSecretStoreRefKind;
const TargetCreationPolicy = crd.ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = crd.ExternalSecretSpecTargetDeletionPolicy;
const ConversionStrategy = crd.ExternalSecretSpecDataRemoteRefConversionStrategy;
const DecodingStrategy = crd.ExternalSecretSpecDataRemoteRefDecodingStrategy;
const MetadataPolicy = crd.ExternalSecretSpecDataRemoteRefMetadataPolicy;
const NullBytePolicy = crd.ExternalSecretSpecDataRemoteRefNullBytePolicy;

/**
 * The full democratic-csi driver config, rendered by ESO so the SSH private
 * key and CHAP credentials never appear in source or manifests. Field
 * placement mirrors the E2E-proven layout (tools/internal/workflow/e2e/
 * e2e_storage_values.go): targetPortal lives under iscsi, block attributes
 * under shareStrategyTargetCli.
 */
export class DriverConfigSecret extends Construct {
  public readonly secretName = DRIVER_CONFIG_SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);
    new crd.ExternalSecret(this, "secret", {
      metadata: { name: DRIVER_CONFIG_SECRET_NAME },
      spec: credentialSecretSpec(DRIVER_CONFIG_SECRET_NAME, {
        "driver-config-file.yaml": driverConfigTemplate(),
      }),
    });
  }
}

/**
 * CHAP credentials in the shape kubelet passes to the iSCSI node stage
 * (csi.storage.k8s.io/node-stage-secret-name on the StorageClass).
 */
export class NodeStageSecret extends Construct {
  public readonly secretName = NODE_STAGE_SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);
    new crd.ExternalSecret(this, "secret", {
      metadata: { name: NODE_STAGE_SECRET_NAME },
      spec: credentialSecretSpec(NODE_STAGE_SECRET_NAME, {
        "node-db.node.session.auth.authmethod": "CHAP",
        "node-db.node.session.auth.username": "{{ .chapUsername }}",
        "node-db.node.session.auth.password": "{{ .chapPassword }}",
        "node-db.node.session.timeo.replacement_timeout": "180",
      }),
    });
  }
}

function credentialSecretSpec(name: string, templateData: Record<string, string>): crd.ExternalSecretSpec {
  return {
    secretStoreRef: { kind: SecretStoreKind.CLUSTER_SECRET_STORE, name: "onepassword" },
    target: credentialSecretTarget(name, templateData),
    data: credentialFields(),
  };
}

function credentialSecretTarget(name: string, templateData: Record<string, string>): crd.ExternalSecretSpecTarget {
  return {
    creationPolicy: TargetCreationPolicy.OWNER,
    deletionPolicy: TargetDeletionPolicy.RETAIN,
    name,
    template: { data: templateData },
  };
}

function credentialFields(): crd.ExternalSecretSpecData[] {
  return ["csiPrivateKey", "chapUsername", "chapPassword"].map(field => ({
    secretKey: field,
    remoteRef: {
      conversionStrategy: ConversionStrategy.DEFAULT,
      decodingStrategy: DecodingStrategy.NONE,
      key: `${CREDENTIALS_ITEM_ID}/${field}`,
      metadataPolicy: MetadataPolicy.NONE,
      nullBytePolicy: NullBytePolicy.IGNORE,
    },
  }));
}

function driverConfigTemplate(): string {
  return `driver: zfs-generic-iscsi
sshConnection:
  host: ${APPLIANCE_FABRIC_ADDRESS}
  port: ${APPLIANCE_SSH_PORT}
  username: k2-csi
  privateKey: {{ .csiPrivateKey | toJson }}
zfs:
  cli:
    sudoEnabled: true
  datasetParentName: ${DATASET_PARENT}
  detachedSnapshotsDatasetParentName: ${DETACHED_SNAPSHOTS_DATASET_PARENT}
  zvolEnableReservation: false
  zvolBlocksize: "16K"
iscsi:
  shareStrategy: targetCli
  shareStrategyTargetCli:
    sudoEnabled: true
    basename: ${IQN_BASE}
    tpg:
      attributes:
        authentication: 1
        generate_node_acls: 1
        cache_dynamic_acls: 1
        demo_mode_write_protect: 0
      auth:
        userid: {{ .chapUsername | toJson }}
        password: {{ .chapPassword | toJson }}
    block:
      attributes:
        emulate_tpu: 1
  targetPortal: ${ISCSI_PORTAL}
`;
}
