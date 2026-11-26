/* eslint-disable k2/no-raw-k8s-jobs -- Exercise the same native Job used by Pomerium's image-provided bootstrap command. */
import assert from "node:assert/strict";
import test from "node:test";

import { Testing } from "cdk8s";
import { Job } from "cdk8s-plus-32";

import { nameBootstrapJob } from "./bootstrap-job-name.js";

function jobName(image: string, args = ["gen-secrets"], label = "original"): string {
  const chart = Testing.chart();
  const job = new Job(chart, "bootstrap", {
    metadata: { name: "pomerium-gen-secrets", labels: { review: label } },
    containers: [{ image, args }],
    select: false,
  });
  nameBootstrapJob(job);
  const rendered = Testing.synth(chart).find(resource => resource.kind === "Job");
  assert.ok(rendered);
  return (rendered.metadata as { name: string }).name;
}

test("bootstrap Job identity is stable for an unchanged template", () => {
  assert.equal(jobName("pomerium/ingress-controller:v0.34.2"), jobName("pomerium/ingress-controller:v0.34.2"));
});

test("immutable image and command changes create distinct bootstrap Jobs", () => {
  const original = jobName("pomerium/ingress-controller:v0.34.1");
  assert.notEqual(original, jobName("pomerium/ingress-controller:v0.34.2"));
  assert.notEqual(
    original,
    jobName("pomerium/ingress-controller:v0.34.1", ["gen-secrets", "--secrets=pomerium/bootstrap"]),
  );
});

test("mutable Job metadata does not replace a completed bootstrap Job", () => {
  assert.equal(jobName("image", undefined, "original"), jobName("image", undefined, "updated"));
});
