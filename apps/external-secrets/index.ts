import type { AppResourceFunc } from "@k2/cdk-lib";

import { ExternalSecrets } from "./components/external-secrets.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { SecretStores } from "./components/secret-stores.js";

export { ManagedSecret, type ManagedSecretProps } from "./lib/managed-secret.js";

export const createAppResources: AppResourceFunc = app => {
  new ExternalSecrets(app, "external-secrets");
  new SecretStores(app, "secret-stores");
  new NetworkPolicy(app, "network-policy");
};
