import assert from "node:assert/strict";
import test from "node:test";

import { Size, Testing } from "cdk8s";

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
