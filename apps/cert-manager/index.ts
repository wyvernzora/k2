import type { AppResourceFunc } from "@k2/cdk-lib";

import { CertManager } from "./components/cert-manager/index.js";
import { DefaultCertificateReplication } from "./components/default-certificate-replication/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * as crd from "./lib/crd.js";

export const createAppResources: AppResourceFunc = app => {
  new CertManager(app, "cert-manager");
  new DefaultCertificateReplication(app, "default-certificate-replication");
  new NetworkPolicy(app, "network-policy");
};
