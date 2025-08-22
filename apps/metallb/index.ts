import { AppResourceFunc, ArgoCDResourceFunc, HelmChartV1 } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import { IpAddressPool, L2Advertisement } from "./crds/metallb.io";

/* Export raw CRDs */
import * as CRD from "./crds/metallb.io";
export const crd = {
  ...CRD,
};

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const chart = new HelmChartV1(app, "metallb", {
    namespace: "k2-network",
    chart: "helm:https://metallb.github.io/metallb/metallb@0.15.2",
  });

  /**
   * Address pool where IP addresses get assigned from by default.
   */
  const defaultAddressPool = new IpAddressPool(chart, "default-pool", {
    spec: {
      addresses: ["10.10.12.1-10.10.12.254"],
    },
  });

  /**
   * Addresses in this pool are accessible from the sandbox VLAN, and are never
   * auto-assigned. Explicitly specify this pool if a service needs to be accessible
   * from the sandbox VLAN.
   */
  const sandboxAddressPool = new IpAddressPool(chart, "sandbox-pool", {
    spec: {
      autoAssign: false,
      addresses: ["10.10.10.0-10.10.10.254"],
    },
  });

  /**
   * Advertise both address pools
   */
  new L2Advertisement(chart, "default", {
    spec: {
      ipAddressPools: [defaultAddressPool.name, sandboxAddressPool.name],
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "metallb", { namespace: "k2-network" });
};
