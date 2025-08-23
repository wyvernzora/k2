import { AppResourceFunc, ArgoCDResourceFunc, HelmCharts, Namespace } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { IpAddressPool, L2Advertisement } from "./crds/metallb.io.js";
export * as crd from "./crds/metallb.io.js";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  app.use(Namespace, "k2-network");
  const Metallb = HelmCharts.of(app).asChart("metallb");

  const chart = new Metallb(app, "metallb", { ...Namespace.of(app) });

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
