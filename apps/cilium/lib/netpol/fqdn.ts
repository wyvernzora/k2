import type { FqdnMatch } from "./types.js";

export const fqdn = {
  name(matchName: string): FqdnMatch {
    return { matchName };
  },
  pattern(matchPattern: string): FqdnMatch {
    return { matchPattern };
  },
};
