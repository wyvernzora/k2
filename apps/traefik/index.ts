/* Export raw CRDs */
import * as CRD from "./crds/traefik.io";
import * as GatewayCRD from "./crds/gateway.networking.k8s.io";
import * as HubCRD from "./crds/hub.traefik.io";

export const crd = {
  ...CRD,
  gateway: GatewayCRD,
  hub: HubCRD,
};

/* Export higher level constructs */
// No constructs

/* Export deployment chart factory */
export * from "./deploy";

/* Export ArgoCD application factory */
// TODO
