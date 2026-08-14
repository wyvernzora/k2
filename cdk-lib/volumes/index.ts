import { K2Volume } from "./base.js";
import { K2EphemeralVolume } from "./ephemeral.js";
import { K2NfsVolume } from "./nfs.js";
import { K2ProvisionedNfsVolume } from "./provisioned-nfs.js";
import { K2IscsiVolume } from "./iscsi.js";
import { K2MigrateVolume } from "./migrate.js";

// Late-bind the static factories declared on K2Volume in base.ts. See the
// JSDoc on K2Volume for why this lives here rather than inside the class.
K2Volume.ephemeral = props => new K2EphemeralVolume(props ?? {});
K2Volume.mountNfs = props => new K2NfsVolume(props);
K2Volume.provisionNfs = props => new K2ProvisionedNfsVolume(props);
K2Volume.iscsi = props => new K2IscsiVolume(props);
K2Volume.migrate = props => new K2MigrateVolume(props);

export * from "./base.js";
export * from "./ephemeral.js";
export * from "./nfs.js";
export * from "./provisioned-nfs.js";
export * from "./iscsi.js";
export * from "./migrate.js";
