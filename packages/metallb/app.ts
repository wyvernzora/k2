import { K2App, HelmChart } from "@k2/cdk-lib";
import { IpAddressPool, L2Advertisement } from "@k2/metallb/crds";

const app = new K2App();
const chart = new HelmChart(app, "metallb", {
  namespace: "k2-network",
  chart: "helm:https://metallb.github.io/metallb/metallb@0.14.5",
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

app.synth();
