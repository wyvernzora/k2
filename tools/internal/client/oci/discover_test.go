package oci

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// fakeRegistry is the in-memory `registry` impl tests use. Each tag
// has a config (with `created`) + a digest. Methods can be made to
// fail for a specific ref by populating the *_err maps.
type fakeRegistry struct {
	tags    map[string][]string       // repo -> tag list
	configs map[string]*v1.ConfigFile // ref -> ConfigFile
	digests map[string]string         // ref -> digest
	rootFS  map[string]uint64         // ref -> measured rootfs MiB
	errors  map[string]error          // ref -> InspectImage error
	listErr error                     // ListTags error
}

func (f *fakeRegistry) ListTags(_ context.Context, repo string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tags[repo], nil
}

func (f *fakeRegistry) Config(_ context.Context, ref string) (*v1.ConfigFile, error) {
	if err, ok := f.errors[ref]; ok && err != nil {
		return nil, err
	}
	cfg, ok := f.configs[ref]
	if !ok {
		return nil, errors.New("not found")
	}
	return cfg, nil
}

func (f *fakeRegistry) Digest(_ context.Context, ref string) (string, error) {
	if err, ok := f.errors[ref]; ok && err != nil {
		return "", err
	}
	d, ok := f.digests[ref]
	if !ok {
		return "", errors.New("not found")
	}
	return d, nil
}

func (f *fakeRegistry) RootFSSizeMiB(_ context.Context, ref string) (uint64, error) {
	if err, ok := f.errors[ref]; ok && err != nil {
		return 0, err
	}
	if size, ok := f.rootFS[ref]; ok {
		return size, nil
	}
	return 1504, nil
}

func cfg(t time.Time) *v1.ConfigFile {
	return &v1.ConfigFile{
		Created: v1.Time{Time: t},
		Config: v1.Config{Labels: map[string]string{
			stateSizeLabel:            "8192",
			upgradeSizeAllowanceLabel: "1536",
		}},
	}
}

func TestDiscoverLatestPicksNewestByCreated(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	repo := "ghcr.io/wyvernzora/k2-kairos"
	prefix := "ubuntu-26.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-"
	fr := &fakeRegistry{
		tags: map[string][]string{
			repo: {
				prefix + "rev0",
				prefix + "rev1",
				prefix + "rev2",
				// Different prefix — must be ignored.
				"ubuntu-26.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev9",
			},
		},
		configs: map[string]*v1.ConfigFile{
			repo + ":" + prefix + "rev0":                                   cfg(now.Add(-72 * time.Hour)),
			repo + ":" + prefix + "rev1":                                   cfg(now.Add(-48 * time.Hour)),
			repo + ":" + prefix + "rev2":                                   cfg(now.Add(-24 * time.Hour)),
			repo + ":ubuntu-26.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev9": cfg(now), // newer but wrong arch
		},
		digests: map[string]string{
			repo + ":" + prefix + "rev0":                                   "sha256:000",
			repo + ":" + prefix + "rev1":                                   "sha256:111",
			repo + ":" + prefix + "rev2":                                   "sha256:222",
			repo + ":ubuntu-26.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev9": "sha256:999",
		},
	}
	d := NewWithRegistry(fr)
	got, err := d.DiscoverLatest(context.Background(), repo, prefix)
	if err != nil {
		t.Fatalf("DiscoverLatest: %v", err)
	}
	want := repo + ":" + prefix + "rev2"
	if got.Ref != want {
		t.Errorf("got Ref=%q want %q", got.Ref, want)
	}
	if got.Digest != "sha256:222" {
		t.Errorf("got Digest=%q want sha256:222", got.Digest)
	}
	if !got.Created.Equal(now.Add(-24 * time.Hour)) {
		t.Errorf("got Created=%v want %v", got.Created, now.Add(-24*time.Hour))
	}
}

func TestDiscoverLatestErrorsWhenPrefixUnmatched(t *testing.T) {
	repo := "ghcr.io/wyvernzora/k2-kairos"
	fr := &fakeRegistry{
		tags: map[string][]string{
			repo: {"ubuntu-26.04-v4.1.0-amd64-qemu-k8s-v1.36.0-k3s1-rev0"},
		},
	}
	d := NewWithRegistry(fr)
	_, err := d.DiscoverLatest(context.Background(), repo, "ubuntu-26.04-arm64-rpi4cb-k8s-")
	if err == nil {
		t.Fatal("expected an error when no tag matches the prefix")
	}
}

func TestDiscoverLatestSkipsTagsThatFailInspection(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	repo := "ghcr.io/wyvernzora/k2-kairos"
	prefix := "ubuntu-26.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-"
	fr := &fakeRegistry{
		tags: map[string][]string{
			repo: {prefix + "rev0", prefix + "rev1"},
		},
		configs: map[string]*v1.ConfigFile{
			repo + ":" + prefix + "rev0": cfg(now.Add(-48 * time.Hour)),
			// rev1 missing from configs → inspection fails.
		},
		digests: map[string]string{
			repo + ":" + prefix + "rev0": "sha256:000",
		},
	}
	d := NewWithRegistry(fr)
	got, err := d.DiscoverLatest(context.Background(), repo, prefix)
	if err != nil {
		t.Fatalf("DiscoverLatest: %v", err)
	}
	if got.Ref != repo+":"+prefix+"rev0" {
		t.Errorf("expected to fall back to the inspectable tag, got Ref=%q", got.Ref)
	}
}

func TestDiscoverLatestErrorsWhenAllTagsFailInspection(t *testing.T) {
	repo := "ghcr.io/wyvernzora/k2-kairos"
	prefix := "ubuntu-26.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-"
	fr := &fakeRegistry{
		tags: map[string][]string{
			repo: {prefix + "rev0", prefix + "rev1"},
		},
		// Both refs missing from configs.
	}
	d := NewWithRegistry(fr)
	_, err := d.DiscoverLatest(context.Background(), repo, prefix)
	if err == nil {
		t.Fatal("expected an error when every matching tag fails inspection")
	}
}

func TestDiscoverLatestPropagatesListTagsError(t *testing.T) {
	fr := &fakeRegistry{listErr: errors.New("registry down")}
	d := NewWithRegistry(fr)
	_, err := d.DiscoverLatest(context.Background(), "ghcr.io/x", "p")
	if err == nil {
		t.Fatal("expected ListTags error to propagate")
	}
}

func TestInspectImageReturnsConfigAndDigest(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	ref := "ghcr.io/wyvernzora/k2-kairos:something"
	fr := &fakeRegistry{
		configs: map[string]*v1.ConfigFile{ref: cfg(now)},
		digests: map[string]string{ref: "sha256:abc"},
	}
	d := NewWithRegistry(fr)
	got, err := d.InspectImage(context.Background(), ref)
	if err != nil {
		t.Fatalf("InspectImage: %v", err)
	}
	if got.Ref != ref || got.Digest != "sha256:abc" || !got.Created.Equal(now) {
		t.Errorf("unexpected Image: %+v", got)
	}
	if got.StateSizeMiB != 8192 || got.UpgradeAllocationMiB != 1504 || got.UpgradeSizeAllowanceMiB != 1536 {
		t.Errorf("unexpected sizing metadata: %+v", got)
	}
}

func TestSingleLinuxImageDescriptorIgnoresProvenanceManifest(t *testing.T) {
	want := v1.Descriptor{
		Digest:   v1.Hash{Algorithm: "sha256", Hex: "abc"},
		Platform: &v1.Platform{OS: "linux", Architecture: "arm64"},
	}
	manifest := &v1.IndexManifest{Manifests: []v1.Descriptor{
		want,
		{Platform: &v1.Platform{OS: "unknown", Architecture: "unknown"}},
	}}

	got, err := singleLinuxImageDescriptor("example.invalid/image:tag", manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != want.Digest {
		t.Fatalf("digest = %s, want %s", got.Digest, want.Digest)
	}
}

func TestSingleLinuxImageDescriptorRejectsMultiArchIndex(t *testing.T) {
	manifest := &v1.IndexManifest{Manifests: []v1.Descriptor{
		{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
		{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}},
	}}

	if _, err := singleLinuxImageDescriptor("example.invalid/image:tag", manifest); err == nil {
		t.Fatal("expected multi-architecture index to be rejected")
	}
}

func TestReadLayerFile(t *testing.T) {
	content := []byte("rootfsSizeMiB: 1504\n")
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{
		Name: imageMetadataPath,
		Mode: 0o644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(archive.Bytes())), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := readLayerFile(layer, imageMetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !bytes.Equal(got, content) {
		t.Fatalf("found=%v content=%q, want %q", found, got, content)
	}
}

func TestInspectImageRequiresSizingLabels(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	ref := "ghcr.io/wyvernzora/k2-kairos:legacy"
	fr := &fakeRegistry{
		configs: map[string]*v1.ConfigFile{ref: {Created: v1.Time{Time: now}}},
		digests: map[string]string{ref: "sha256:abc"},
	}

	_, err := NewWithRegistry(fr).InspectImage(context.Background(), ref)
	if err == nil || !strings.Contains(err.Error(), stateSizeLabel) {
		t.Fatalf("error = %v, want missing state size label", err)
	}
}
