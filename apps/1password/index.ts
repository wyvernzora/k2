/* Export raw CRDs */
import * as OnePasswordCRD from "./crds/onepassword.com";
export const crd = {
  ...OnePasswordCRD,
};

/* Export higher level constructs */
export * from "./constructs/item";
export * from "./constructs/context";

/* Export deployment chart factory */
export * from "./deploy";

/* Export ArgoCD application factory */
// TODO
