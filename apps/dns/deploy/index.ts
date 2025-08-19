import { App } from "@k2/cdk-lib";
import { BlockingGroup, ClientGroup, CustomDns, Dns } from "../components";

const app = new App();

// Default client group and its blocking config
const blockingGroup = new BlockingGroup({
  name: "default",
  blacklists: ["https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"],
});
const clientGroup = new ClientGroup({
  name: "default",
  blockingGroups: [blockingGroup],
  upstream: ["10.10.1.1"],
});

// Custom DNS
const customDns = new CustomDns({
  records: {
    unifi: ["10.10.1.1"],
    roxy: ["10.10.7.1"],
    eris: ["10.10.7.2"],
    sylphy: ["10.10.7.3"],
    pve: ["10.10.7.254"],
    rumi: ["10.10.8.1"],
    k8s: ["10.10.8.2"],
  },
});

// Create DNS chart
new Dns(app, "dns", {
  namespace: "k2-network",
  apexDomain: "wyvernzora.io",
  serviceIp: "10.10.10.8",
  clientGroups: [clientGroup],
  customDns,
});

app.synth();
