package oci

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// fakeRegistry is the in-memory `registry` impl tests use. Each tag
// has a config (with `created`) + a digest. Methods can be made to
// fail for a specific ref by populating the *_err maps.
type fakeRegistry struct {
	tags    map[string][]string       // repo -> tag list
	configs map[string]*v1.ConfigFile // ref -> ConfigFile
	digests map[string]string         // ref -> digest
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

func cfg(t time.Time) *v1.ConfigFile {
	return &v1.ConfigFile{Created: v1.Time{Time: t}}
}

func TestDiscoverLatestPicksNewestByCreated(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	repo := "ghcr.io/wyvernzora/k2-kairos"
	prefix := "ubuntu-24.04-standard-arm64-rpi4cb-k3s-v1.36.0-k3s1-"
	fr := &fakeRegistry{
		tags: map[string][]string{
			repo: {
				prefix + "rev0",
				prefix + "rev1",
				prefix + "rev2",
				// Different prefix — must be ignored.
				"ubuntu-24.04-standard-amd64-qemu-k3s-v1.36.0-k3s1-rev9",
			},
		},
		configs: map[string]*v1.ConfigFile{
			repo + ":" + prefix + "rev0":                                     cfg(now.Add(-72 * time.Hour)),
			repo + ":" + prefix + "rev1":                                     cfg(now.Add(-48 * time.Hour)),
			repo + ":" + prefix + "rev2":                                     cfg(now.Add(-24 * time.Hour)),
			repo + ":ubuntu-24.04-standard-amd64-qemu-k3s-v1.36.0-k3s1-rev9": cfg(now), // newer but wrong arch
		},
		digests: map[string]string{
			repo + ":" + prefix + "rev0":                                     "sha256:000",
			repo + ":" + prefix + "rev1":                                     "sha256:111",
			repo + ":" + prefix + "rev2":                                     "sha256:222",
			repo + ":ubuntu-24.04-standard-amd64-qemu-k3s-v1.36.0-k3s1-rev9": "sha256:999",
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
			repo: {"ubuntu-24.04-standard-amd64-qemu-k3s-v1.36.0-k3s1-rev0"},
		},
	}
	d := NewWithRegistry(fr)
	_, err := d.DiscoverLatest(context.Background(), repo, "ubuntu-24.04-standard-arm64-rpi4cb-k3s-")
	if err == nil {
		t.Fatal("expected an error when no tag matches the prefix")
	}
}

func TestDiscoverLatestSkipsTagsThatFailInspection(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	repo := "ghcr.io/wyvernzora/k2-kairos"
	prefix := "ubuntu-24.04-standard-arm64-rpi4cb-k3s-v1.36.0-k3s1-"
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
	prefix := "ubuntu-24.04-standard-arm64-rpi4cb-k3s-v1.36.0-k3s1-"
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
}
