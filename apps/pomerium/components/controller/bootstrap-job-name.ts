import { createHash } from "node:crypto";

import { ApiObject, JsonPatch } from "cdk8s";
import type { Job } from "cdk8s-plus-32";
import stringify from "json-stable-stringify";

export function nameBootstrapJob(job: Job): void {
  const api = ApiObject.of(job);
  const rendered = api.toJson() as { spec: { template: object } };
  const template = stringify(rendered.spec.template);
  if (template === undefined) {
    throw new Error("Bootstrap Job must render a pod template");
  }
  const digest = createHash("sha256").update(template).digest("hex").slice(0, 12);
  api.addJsonPatch(JsonPatch.add("/metadata/name", `pomerium-gen-secrets-${digest}`));
}
