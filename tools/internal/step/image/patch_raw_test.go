package image

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wyvernzora/k2/tools/internal/image/plan"
)

func TestPreparePartitionWork(t *testing.T) {
	work, err := preparePartitionWork([]plan.RawPatch{
		{
			Type:           "copy-to-partition",
			PartitionLabel: "COS_GRUB",
			Path:           "extraconfig.txt",
		},
		{
			Type:           "yaml-json-patch",
			PartitionLabel: "COS_OEM",
			TargetPath:     "01_reset.yaml",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(work) != 2 {
		t.Fatalf("work entries = %#v, want 2", work)
	}
	if work[0].Label != "COS_GRUB" || len(work[0].ExtractPaths) != 0 {
		t.Fatalf("first work entry = %#v, want COS_GRUB with no extract paths", work[0])
	}
	if work[1].Label != "COS_OEM" || len(work[1].ExtractPaths) != 1 || work[1].ExtractPaths[0] != "01_reset.yaml" {
		t.Fatalf("second work entry = %#v, want COS_OEM extracting 01_reset.yaml", work[1])
	}
}

func TestValidatePartitionPath(t *testing.T) {
	for _, path := range []string{"", "/absolute", "../escape", "dir/../../escape", "bad\npath"} {
		t.Run(path, func(t *testing.T) {
			if err := validatePartitionPath(path); err == nil {
				t.Fatal("expected invalid path error")
			}
		})
	}

	if err := validatePartitionPath("dir/file.yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestPartitionHelperExtractMountsReadOnly(t *testing.T) {
	if !strings.Contains(partitionHelperScript, `if [ "${PATCH_MODE}" = "extract" ]; then`) {
		t.Fatal("partition helper does not branch on extract mode")
	}
	if !strings.Contains(partitionHelperScript, `mount -o ro "${loopdev}" "${mount_dir}"`) {
		t.Fatal("partition helper extract mode does not mount read-only")
	}
}

func TestWaitForHostVisibleRawChangeHonorsCancellation(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "image.raw")
	if err := os.WriteFile(raw, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := rawPatchSHA256File(raw)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = waitForHostVisibleRawChange(ctx, raw, info.ModTime(), hash, time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestWaitForHostVisibleRawChangeAllowsSettledNoopRewrite(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "image.raw")
	if err := os.WriteFile(raw, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := rawPatchSHA256File(raw)
	if err != nil {
		t.Fatal(err)
	}
	previousModTime := time.Now().Add(-2 * time.Minute)
	settledRewriteTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(raw, settledRewriteTime, settledRewriteTime); err != nil {
		t.Fatal(err)
	}

	if err := waitForHostVisibleRawChange(context.Background(), raw, previousModTime, hash, time.Minute); err != nil {
		t.Fatalf("waitForHostVisibleRawChange: %v", err)
	}
}
