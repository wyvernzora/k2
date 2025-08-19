/* Export raw CRDs */
import * as OnePasswordCRD from "./crds/onepassword.com";
export const crd = {
  ...OnePasswordCRD,
};

/* Export higher level constructs */
export * from "./lib/item";
export * from "./lib/context";

/* Export deployment chart factory */
export * from "./deploy";

/* Export ArgoCD application factory */
// TODO
