import type { FqdnMatch } from "./types.js";

export function fqdnMatch(match: string | FqdnMatch): FqdnMatch {
  if (typeof match === "string") {
    return { matchName: match };
  }
  if ((match.matchName === undefined) === (match.matchPattern === undefined)) {
    throw new Error("FqdnMatch must set exactly one of matchName or matchPattern");
  }
  return match;
}
