import assert from "node:assert/strict";
import test from "node:test";

import { Size, Testing } from "cdk8s";
import { Deployment } from "cdk8s-plus-32";

import { K2Volume } from "./index.js";

const SIZE = Size.gibibytes(4);
const ID = "vol-appdata";

function claims(volume: K2Volume): Record<string, unknown>[] {
  const chart = Testing.chart();
  volume.materialize(chart, ID);
  return Testing.synth(chart).filter(object => object.kind === "PersistentVolumeClaim");
}

function nameOf(claim: Record<string, unknown>): string {
  return (claim.metadata as { name: string }).name;
}

function storageClassOf(claim: Record<string, unknown>): string {
  return (claim.spec as { storageClassName: string }).storageClassName;
}

function migrating(): Record<string, unknown>[] {
  return claims(
    K2Volume.migrate({
      from: K2Volume.replicated({ size: SIZE }),
      to: K2Volume.iscsi({ size: SIZE }),
    }),
  );
}

// The three properties that make `v -> migrate(v, w) -> w` safe as a pure
// sequence of code edits. Each one, if broken, silently lands the app on an
// empty volume rather than failing loudly.

test("step 1: wrapping emits the source claim unchanged, so the live PVC is adopted", () => {
  const before = claims(K2Volume.replicated({ size: SIZE }));
  const wrapped = migrating();

  assert.equal(before.length, 1);
  assert.equal(wrapped.length, 2, "migrate emits both claims");
  assert.deepEqual(
    wrapped.find(claim => nameOf(claim) === nameOf(before[0])),
    before[0],
    "migrate({from: v}) must emit v's PVC verbatim",
  );
});

test("step 2: unwrapping emits the destination claim unchanged, so the data is kept", () => {
  const wrapped = migrating();
  const after = claims(K2Volume.iscsi({ size: SIZE }));

  assert.equal(after.length, 1);
  assert.deepEqual(
    wrapped.find(claim => nameOf(claim) === nameOf(after[0])),
    after[0],
    "the destination must be named as if declared directly, or unwrapping abandons the migrated volume",
  );
});

test("the two claims coexist under distinct names while the copy runs", () => {
  const wrapped = migrating();
  assert.notEqual(nameOf(wrapped[0]), nameOf(wrapped[1]));
  assert.deepEqual(wrapped.map(storageClassOf).sort(), ["k2-iscsi", "longhorn"], "one claim per storage class");
});

// Naming must be reproducible across synths: it is what ties a deployed PVC to
// the code that declared it.
test("destination naming is deterministic and free of hand-written names", () => {
  assert.equal(nameOf(claims(K2Volume.iscsi({ size: SIZE }))[0]), nameOf(claims(K2Volume.iscsi({ size: SIZE }))[0]));
  assert.match(nameOf(claims(K2Volume.iscsi({ size: SIZE }))[0]), /^[a-z0-9-]+$/);
});

// Same-driver migration works as long as the classes differ, because the claim
// name is derived from the storage class.
test("migrating between two classes of the same driver keeps the claims distinct", () => {
  const chart = Testing.chart();
  K2Volume.migrate({
    from: K2Volume.iscsi({ size: SIZE }),
    to: K2Volume.iscsi({ size: SIZE, storageClass: "k2-iscsi-nvme" }),
  }).materialize(chart, ID);
  const emitted = Testing.synth(chart).filter(object => object.kind === "PersistentVolumeClaim");

  assert.equal(emitted.length, 2);
  assert.notEqual(nameOf(emitted[0]), nameOf(emitted[1]));
});

// Within a single class there is nothing left to distinguish them, and two
// claims sharing a name would silently apply over each other.
test("migrating within one storage class fails loudly rather than colliding", () => {
  assert.throws(
    () =>
      K2Volume.migrate({
        from: K2Volume.iscsi({ size: SIZE }),
        to: K2Volume.iscsi({ size: Size.gibibytes(8) }),
      }).materialize(Testing.chart(), ID),
    /single storage class is not supported/,
  );
});

// Every other claim in this repo is declared with an explicit name, so this is
// the path the remaining migrations take: an explicit name is used verbatim and
// is unaffected by being wrapped, leaving nothing about naming to get wrong.
test("an explicitly named claim is identical wrapped or not", () => {
  const named = (): K2Volume => K2Volume.replicated({ name: "n8n-appdata", size: SIZE });
  const standalone = claims(named());
  const wrapped = claims(K2Volume.migrate({ from: named(), to: K2Volume.iscsi({ size: SIZE }) }));

  assert.equal(nameOf(standalone[0]), "n8n-appdata");
  assert.deepEqual(
    wrapped.find(claim => nameOf(claim) === "n8n-appdata"),
    standalone[0],
    "wrapping must not alter an explicitly named claim",
  );
});

// The marker is a file on the destination, so it OUTLIVES the migration that
// wrote it: unwrapping leaves it behind forever. A marker that only says "a
// migration finished here" therefore suppresses the copy for any LATER
// migration into the same volume, which reports success while leaving the old
// data in place. Naming the source in the marker is what keeps that
// unrepresentable rather than merely documented.
test("the completion marker identifies its source, so a stale one cannot suppress a later migration", () => {
  const script = (from: K2Volume): string => {
    const chart = Testing.chart();
    const deployment = new Deployment(chart, "workload", { containers: [{ image: "app" }] });
    K2Volume.migrate({ from, to: K2Volume.iscsi({ size: SIZE }) })
      .materialize(chart, ID)
      .configureWorkload?.(deployment);
    const synthed = Testing.synth(chart).find(object => object.kind === "Deployment");
    const spec = synthed?.spec as { template: { spec: { initContainers: { command: string[] }[] } } };
    return spec.template.spec.initContainers[0].command[2];
  };

  const fromLonghorn = script(K2Volume.replicated({ size: SIZE }));
  const fromElsewhere = script(K2Volume.replicated({ name: "some-other-source", size: SIZE }));

  assert.notEqual(
    fromLonghorn,
    fromElsewhere,
    "two different sources must not share a completion check, or a marker left by one skips the other",
  );
  assert.match(fromElsewhere, /some-other-source/, "the completion check must name the source it is asserting about");
});

// initContainers is a merge-key list keyed on `name`, so duplicates do not fail
// loudly: kubectl apply collapses them and the API server validates the result,
// which means N migrations in one workload silently become ONE and the other
// N-1 volumes mount empty. Migrating every volume of an app at once is the
// obvious way to use this, so the default name has to be per-volume.
test("each migration in a workload gets its own init container", () => {
  const chart = Testing.chart();
  const deployment = new Deployment(chart, "workload", { containers: [{ image: "app" }] });
  // camelCase keys are the norm for a volumes record, and cdk8s does not
  // sanitize a container name the way it does a resource name, so the id
  // reaches the manifest verbatim and must be made a legal label here.
  for (const id of ["vol-appdata", "vol-codexHome"]) {
    K2Volume.migrate({
      from: K2Volume.replicated({ name: `src-${id}`, size: SIZE }),
      to: K2Volume.iscsi({ size: SIZE }),
    })
      .materialize(chart, id)
      .configureWorkload?.(deployment);
  }

  const synthed = Testing.synth(chart).find(object => object.kind === "Deployment");
  const spec = synthed?.spec as { template: { spec: { initContainers: { name: string }[] } } };
  const names = spec.template.spec.initContainers.map(container => container.name);

  assert.equal(new Set(names).size, names.length, `init container names must be unique, got ${names.join(", ")}`);
  for (const name of names) {
    assert.match(name, /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, `${name} is not a valid RFC 1123 label`);
  }
});

// The hash makes a clash unlikely, not impossible, and a caller can always
// supply the same name twice. Since nothing downstream reports the duplicate,
// the only place it can surface is here.
test("a duplicate init container name fails at synth rather than at apply", () => {
  const chart = Testing.chart();
  const deployment = new Deployment(chart, "workload", { containers: [{ image: "app" }] });
  const add = (id: string): void => {
    K2Volume.migrate({
      from: K2Volume.replicated({ name: `src-${id}`, size: SIZE }),
      to: K2Volume.iscsi({ size: SIZE }),
      initContainerName: "shared-name",
    })
      .materialize(chart, id)
      .configureWorkload?.(deployment);
  };

  add("vol-first");
  assert.throws(() => {
    add("vol-second");
  }, /already on this workload/);
});
