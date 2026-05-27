package rawpatch

import (
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
)

func TestApplyJSONPatch(t *testing.T) {
	data := []byte(`
stages:
  rootfs.before:
    - layout:
        add_partitions:
          - fsLabel: COS_STATE
            size: 8192
          - fsLabel: COS_PERSISTENT
            size: 0
`)
	operations := []plan.JSONPatchOperation{
		{
			Op:    "test",
			Path:  "/stages/rootfs.before/0/layout/add_partitions/1/fsLabel",
			Value: "COS_PERSISTENT",
		},
		{
			Op:    "replace",
			Path:  "/stages/rootfs.before/0/layout/add_partitions/1/size",
			Value: 500,
		},
	}

	patched, err := ApplyJSONPatch(data, operations)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), "size: 500") {
		t.Fatalf("patched YAML does not contain size 500:\n%s", patched)
	}
	if err := ValidateJSONPatchResult(patched, operations); err != nil {
		t.Fatal(err)
	}
}

func TestApplyJSONPatchAddsMissingObjectKey(t *testing.T) {
	data := []byte(`
stages:
  rootfs.before:
    - layout:
        add_partitions:
          - fsLabel: COS_STATE
            size: 8192
          - fsLabel: COS_PERSISTENT
`)
	operations := []plan.JSONPatchOperation{
		{
			Op:    "test",
			Path:  "/stages/rootfs.before/0/layout/add_partitions/1/fsLabel",
			Value: "COS_PERSISTENT",
		},
		{
			Op:    "add",
			Path:  "/stages/rootfs.before/0/layout/add_partitions/1/size",
			Value: 500,
		},
	}

	patched, err := ApplyJSONPatch(data, operations)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), "size: 500") {
		t.Fatalf("patched YAML does not contain size 500:\n%s", patched)
	}
	if err := ValidateJSONPatchResult(patched, operations); err != nil {
		t.Fatal(err)
	}
}

func TestApplyJSONPatchFailsTest(t *testing.T) {
	_, err := ApplyJSONPatch([]byte("name: wrong\n"), []plan.JSONPatchOperation{
		{Op: "test", Path: "/name", Value: "expected"},
	})
	if err == nil {
		t.Fatal("expected failed test")
	}
}

func TestApplyJSONPatchRejectsMissingReplacePath(t *testing.T) {
	_, err := ApplyJSONPatch([]byte("name: value\n"), []plan.JSONPatchOperation{
		{Op: "replace", Path: "/missing", Value: "new"},
	})
	if err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestApplyJSONPatchRejectsUnsupportedOperation(t *testing.T) {
	_, err := ApplyJSONPatch([]byte("name: value\n"), []plan.JSONPatchOperation{
		{Op: "remove", Path: "/name"},
	})
	if err == nil {
		t.Fatal("expected unsupported op error")
	}
}

func TestApplyStructuredPatchJSON(t *testing.T) {
	patched, err := ApplyStructuredPatch([]byte(`{"items":[{"name":"old"}]}`), "config.json", []plan.JSONPatchOperation{
		{Op: "test", Path: "/items/0/name", Value: "old"},
		{Op: "replace", Path: "/items/0/name", Value: "new"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), `"name": "new"`) {
		t.Fatalf("patched JSON does not contain replacement:\n%s", patched)
	}
}
