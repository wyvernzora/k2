import { K2Volume } from "./base.js";
import { K2EphemeralVolume } from "./ephemeral.js";
import { K2NfsVolume } from "./nfs.js";
import { K2ReplicatedVolume } from "./replicated.js";

// Late-bind the static factories declared on K2Volume in base.ts. See the
// JSDoc on K2Volume for why this lives here rather than inside the class.
K2Volume.ephemeral = props => new K2EphemeralVolume(props ?? {});
K2Volume.nfs = props => new K2NfsVolume(props);
K2Volume.replicated = props => new K2ReplicatedVolume(props);

export * from "./base.js";
export * from "./ephemeral.js";
export * from "./nfs.js";
export * from "./replicated.js";
