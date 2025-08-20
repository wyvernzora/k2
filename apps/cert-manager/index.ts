/* Export raw CRDs */
import * as AcmeCRD from "./crds/acme.cert-manager.io";
import * as MainCRD from "./crds/cert-manager.io";
export const crd = {
  ...MainCRD,
  acme: AcmeCRD,
};

/* Export higher level constructs */
export * from "./lib/issuer";
export * from "./lib/certificate";

/* Export deployment chart factory */

/* Export ArgoCD application factory */
// TODO
