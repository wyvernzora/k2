import { OnePasswordItem } from "@k2/1password/crds";
import { ApiObjectMetadata } from "cdk8s";
import { Construct } from "constructs";

const VAULT_ID = "zfsyjjcwge4w4gw6dh4zaqndhq";

export interface K2SecretProps {
  readonly metadata?: ApiObjectMetadata;
  readonly itemId: string;
}

/**
 * K2 cluster uses 1Password to inject secrets.
 * This is a more concise wrapper since all K2 secrets live in a
 * dedicated 1Password vault for isolation.
 */
export class K2Secret extends OnePasswordItem {
  constructor(scope: Construct, id: string, props: K2SecretProps) {
    super(scope, id, {
      metadata: props.metadata,
      spec: {
        itemPath: `vaults/${VAULT_ID}/items/${props.itemId}`,
      },
    });
  }
}
