import type { ApiObjectMetadata } from "cdk8s";
import { type ISecret, Secret } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { ExternalSecret, ExternalSecretSpecSecretStoreRefKind } from "../crds/external-secrets.io.js";

/**
 * Cluster-wide default `ClusterSecretStore` name. Provisioned by
 * `apps/external-secrets/components/external-secrets.ts`. Keeping the
 * constant private to this module — if it ever changes, both sides change
 * together.
 */
const DEFAULT_STORE_NAME = "onepassword";

export interface K2SecretProps {
  readonly metadata?: ApiObjectMetadata;

  /**
   * 1Password item title (or id) within the cluster's 1Password vault.
   * Vault scope is set by `cluster.onePassword.vault` on the
   * `ClusterSecretStore`; this only names the item inside that vault.
   *
   * @example "plex"
   * @example "cert-manager-route53"
   */
  readonly item: string;

  /**
   * Fields to pull from the 1Password item. Map *keys* become Kubernetes
   * Secret keys (so they must satisfy `[-._a-zA-Z0-9]+`); *values* are
   * field paths inside the 1Password item, in the ESO 1Password SDK
   * provider format:
   *
   *   `<field>`              — top-level field, e.g. `"password"`
   *   `<section>/<field>`    — field inside a section, e.g. `"aws/access-key-id"`
   *
   * No `op://` URI prefix; the vault is set by the store, not the ref.
   *
   * @example { token: "token", claim: "claim-token" }
   * @example { accessKeyId: "aws/access-key-id", secretAccessKey: "aws/secret-access-key" }
   */
  readonly fields: Record<string, string>;

  /**
   * Refresh interval as a Go duration string. Defaults to ESO's own
   * default (`1h0m0s`). Pass `"0s"` to fetch once at creation and never
   * refresh — useful for secrets that never rotate (e.g. signing keys).
   */
  readonly refreshInterval?: string;
}

/**
 * K2 secret — concise wrapper over `ExternalSecret` that pre-fills the
 * cluster's default 1Password `ClusterSecretStore`.
 *
 * Produces:
 *   - An `ExternalSecret` resource pointing at the `onepassword` CSS.
 *   - A Kubernetes `Secret` (managed by ESO) with one key per entry in
 *     `props.fields`. The Secret's name equals `metadata.name`, defaulting
 *     to the construct id when `metadata.name` is absent. Reference it via
 *     `.secret` (an `ISecret`) when wiring cdk8s-plus containers
 *     (`envFrom`, `envValueFrom`, mount).
 *
 * Cluster-side prerequisites: the 1Password service-account token Secret
 * must exist in the `external-secrets` namespace (created out-of-band by
 * the provisioner CLI). See `apps/external-secrets/components/`.
 */
export class K2Secret extends ExternalSecret {
  public readonly secret: ISecret;

  public constructor(scope: Construct, id: string, props: K2SecretProps) {
    const name = props.metadata?.name ?? id;
    super(scope, id, {
      metadata: { ...props.metadata, name },
      spec: {
        secretStoreRef: {
          kind: ExternalSecretSpecSecretStoreRefKind.CLUSTER_SECRET_STORE,
          name: DEFAULT_STORE_NAME,
        },
        refreshInterval: props.refreshInterval,
        target: { name },
        data: Object.entries(props.fields).map(([secretKey, fieldPath]) => ({
          secretKey,
          remoteRef: { key: `${props.item}/${fieldPath}` },
        })),
      },
    });
    this.secret = Secret.fromSecretName(this, "secret", name);
  }
}
