import type { ApiObjectMetadata } from "cdk8s";
import { type ISecret, Secret as KubeSecret } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import {
  ExternalSecret,
  ExternalSecretSpecDataRemoteRefConversionStrategy,
  ExternalSecretSpecDataRemoteRefDecodingStrategy,
  ExternalSecretSpecDataRemoteRefMetadataPolicy,
  ExternalSecretSpecDataRemoteRefNullBytePolicy,
  ExternalSecretSpecSecretStoreRefKind,
  ExternalSecretSpecTargetCreationPolicy,
  ExternalSecretSpecTargetDeletionPolicy,
} from "../crds/external-secrets.io.js";

/**
 * Cluster-wide default `ClusterSecretStore` name. Provisioned by
 * `apps/external-secrets/components/secret-stores.ts`. Keeping the
 * constant private to this module means consumer apps do not learn which
 * backend currently stores ordinary app secrets.
 */
const DEFAULT_STORE_NAME = "onepassword";
const SecretStoreKind = ExternalSecretSpecSecretStoreRefKind;
const RemoteRefConversionStrategy = ExternalSecretSpecDataRemoteRefConversionStrategy;
const RemoteRefDecodingStrategy = ExternalSecretSpecDataRemoteRefDecodingStrategy;
const RemoteRefMetadataPolicy = ExternalSecretSpecDataRemoteRefMetadataPolicy;
const RemoteRefNullBytePolicy = ExternalSecretSpecDataRemoteRefNullBytePolicy;
const TargetCreationPolicy = ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = ExternalSecretSpecTargetDeletionPolicy;

export interface ManagedSecretProps {
  readonly metadata?: ApiObjectMetadata;

  /**
   * Logical secret name. The external-secrets library maps this to the active
   * backing provider. Today that means a 1Password item inside the configured
   * vault; future backends should preserve this app-facing contract.
   *
   * @example "plex"
   * @example "cert-manager-route53"
   */
  readonly secret: string;

  /**
   * Fields to materialize into the target Kubernetes Secret. Map keys become
   * Kubernetes Secret keys; values are logical field paths inside the backing
   * secret record.
   *
   * The initial 1Password backend accepts:
   *   `<field>`              - top-level field, e.g. `"password"`
   *   `<section>/<field>`    - field in a section, e.g. `"aws/access-key-id"`
   *
   * @example { token: "token", claim: "claim-token" }
   * @example { accessKeyId: "aws/access-key-id", secretAccessKey: "aws/secret-access-key" }
   */
  readonly fields: Record<string, string>;

  /**
   * Refresh interval as a Go duration string. Defaults to ESO's own default
   * (`1h0m0s`). Pass `"0s"` to fetch once at creation and never refresh.
   */
  readonly refreshInterval?: string;
}

/**
 * Backend-neutral app secret wrapper.
 *
 * Produces:
 *   - An ESO `ExternalSecret` using the current default backend.
 *   - A Kubernetes `Secret` managed by ESO with one key per `props.fields`.
 *
 * Consumer apps should depend on this construct, not on the backing provider.
 */
export class ManagedSecret extends ExternalSecret {
  public readonly secret: ISecret;

  public constructor(scope: Construct, id: string, props: ManagedSecretProps) {
    const name = props.metadata?.name ?? id;
    super(scope, id, {
      metadata: { ...props.metadata, name },
      spec: {
        secretStoreRef: {
          kind: SecretStoreKind.CLUSTER_SECRET_STORE,
          name: DEFAULT_STORE_NAME,
        },
        refreshInterval: props.refreshInterval,
        target: {
          creationPolicy: TargetCreationPolicy.OWNER,
          deletionPolicy: TargetDeletionPolicy.RETAIN,
          name,
        },
        data: Object.entries(props.fields).map(([secretKey, fieldPath]) => ({
          secretKey,
          remoteRef: {
            conversionStrategy: RemoteRefConversionStrategy.DEFAULT,
            decodingStrategy: RemoteRefDecodingStrategy.NONE,
            key: `${props.secret}/${fieldPath}`,
            metadataPolicy: RemoteRefMetadataPolicy.NONE,
            nullBytePolicy: RemoteRefNullBytePolicy.IGNORE,
          },
        })),
      },
    });
    this.secret = KubeSecret.fromSecretName(this, "secret", name);
  }
}
