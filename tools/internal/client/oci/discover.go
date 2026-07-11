// Package oci wraps go-containerregistry to discover the newest
// pushed image of a Kairos OCI build target. The k2-tools upgrade
// subcommand uses it to auto-resolve "the latest image for this
// node's hardware/arch combo" without the operator having to type
// the full ghcr.io tag.
//
// The package isolates the registry-client dependency to one
// internal package so tests anywhere else in the tree don't pull
// in HTTP. The exported Discoverer uses a small interface
// (`registry`) so tests can supply an in-memory fake instead of
// hitting a real registry.
package oci

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gopkg.in/yaml.v3"
)

// Image is the per-tag fact set k2-tools upgrade actually wires into
// its Plan section: the fully qualified OCI ref, the resolved digest
// (so the operator pins by content even when calling kairos-agent
// upgrade with the tag), and the registry-side creation time used
// for the "published N days ago" line.
type Image struct {
	Ref                     string
	Digest                  string
	Created                 time.Time
	StateSizeMiB            uint64
	UpgradeAllocationMiB    uint64
	UpgradeSizeAllowanceMiB uint64
}

const (
	stateSizeLabel            = "io.k2.disk-state-size-mib"
	upgradeSizeAllowanceLabel = "io.k2.upgrade-size-allowance-mib"
	imageMetadataPath         = "usr/share/k2/image-build/metadata.yaml"
	maxImageMetadataBytes     = int64(1 << 20)
)

// Discoverer is the public surface. Hold one for the duration of an
// upgrade invocation; safe for concurrent use only insofar as the
// underlying registry client is (crane is, but we don't rely on it).
type Discoverer struct {
	reg registry
}

// New returns a Discoverer backed by the real crane registry client.
// Tests use NewWithRegistry to swap in a fake.
func New() *Discoverer {
	return &Discoverer{reg: craneRegistry{}}
}

// NewWithRegistry constructs a Discoverer with a custom registry
// implementation; exported only so the test package can inject a
// fake registry. Production code uses New().
func NewWithRegistry(reg registry) *Discoverer {
	return &Discoverer{reg: reg}
}

// InspectImage resolves a single OCI reference to digest + created
// timestamp. Used when the operator passes --source <ref> explicitly
// — we still want the publish-age line in the Plan so they can
// sanity-check what they typed.
func (d *Discoverer) InspectImage(ctx context.Context, ref string) (Image, error) {
	cfg, err := d.reg.Config(ctx, ref)
	if err != nil {
		return Image{}, fmt.Errorf("read OCI config for %s: %w", ref, err)
	}
	digest, err := d.reg.Digest(ctx, ref)
	if err != nil {
		return Image{}, fmt.Errorf("resolve digest for %s: %w", ref, err)
	}
	stateSizeMiB, err := positiveLabel(cfg.Config.Labels, stateSizeLabel)
	if err != nil {
		return Image{}, fmt.Errorf("inspect %s: %w", ref, err)
	}
	upgradeAllocationMiB, err := d.reg.RootFSSizeMiB(ctx, ref)
	if err != nil {
		return Image{}, fmt.Errorf("read rootfs size metadata for %s: %w", ref, err)
	}
	upgradeSizeAllowanceMiB, err := positiveLabel(cfg.Config.Labels, upgradeSizeAllowanceLabel)
	if err != nil {
		return Image{}, fmt.Errorf("inspect %s: %w", ref, err)
	}
	return Image{
		Ref:                     ref,
		Digest:                  digest,
		Created:                 cfg.Created.Time,
		StateSizeMiB:            stateSizeMiB,
		UpgradeAllocationMiB:    upgradeAllocationMiB,
		UpgradeSizeAllowanceMiB: upgradeSizeAllowanceMiB,
	}, nil
}

func positiveLabel(labels map[string]string, name string) (uint64, error) {
	value := strings.TrimSpace(labels[name])
	if value == "" {
		return 0, fmt.Errorf("OCI label %s is required", name)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("OCI label %s must be a positive integer, got %q", name, value)
	}
	return parsed, nil
}

// DiscoverLatest scans `repo` for tags starting with `prefix`,
// inspects each one's image config for its created timestamp, and
// returns the one published most recently. The prefix exists because
// our published-image tag scheme is
// `<target>-<k3sVersion>-rev<N>`, and an operator on rpi4cb does
// NOT want to match a worker's qemu image as "latest." Callers are
// expected to build the prefix from the running node's metadata.
//
// Returns an error if no tags match the prefix; the caller should
// surface this as "no published images matched" rather than falling
// back to anything.
func (d *Discoverer) DiscoverLatest(ctx context.Context, repo, prefix string) (Image, error) {
	tags, err := d.reg.ListTags(ctx, repo)
	if err != nil {
		return Image{}, fmt.Errorf("list tags for %s: %w", repo, err)
	}
	var matched []string
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) {
			matched = append(matched, t)
		}
	}
	if len(matched) == 0 {
		return Image{}, fmt.Errorf("no tags in %s match prefix %q", repo, prefix)
	}
	candidates := make([]Image, 0, len(matched))
	for _, t := range matched {
		ref := repo + ":" + t
		img, err := d.InspectImage(ctx, ref)
		if err != nil {
			// One bad tag doesn't sink the search — the operator
			// frequently has a half-pushed tag at HEAD of a CI run.
			// Log via the error chain (caller can wrap), keep going.
			continue
		}
		candidates = append(candidates, img)
	}
	if len(candidates) == 0 {
		return Image{}, fmt.Errorf("none of the %d tags matching %q in %s could be inspected",
			len(matched), prefix, repo)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Created.After(candidates[j].Created)
	})
	return candidates[0], nil
}

// registry is the seam between this package and the underlying OCI
// client library. Production wires it to crane; tests wire it to an
// in-memory fake. Methods are the minimum subset needed by Discoverer.
//
// All methods take a context for cancellation hygiene.
type registry interface {
	ListTags(ctx context.Context, repo string) ([]string, error)
	Config(ctx context.Context, ref string) (*v1.ConfigFile, error)
	Digest(ctx context.Context, ref string) (string, error)
	RootFSSizeMiB(ctx context.Context, ref string) (uint64, error)
}

// craneRegistry is the production registry impl backed by crane.
// All methods are anonymous-public-read against ghcr.io (which is what
// our k2-kairos images live behind); no auth is configured.
type craneRegistry struct{}

func (craneRegistry) ListTags(ctx context.Context, repo string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return crane.ListTags(repo)
}

func (craneRegistry) Config(ctx context.Context, ref string) (*v1.ConfigFile, error) {
	img, err := pullSinglePlatformImage(ctx, ref)
	if err != nil {
		return nil, err
	}
	return img.ConfigFile()
}

func (craneRegistry) Digest(ctx context.Context, ref string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return crane.Digest(ref)
}

func (craneRegistry) RootFSSizeMiB(ctx context.Context, ref string) (uint64, error) {
	img, err := pullSinglePlatformImage(ctx, ref)
	if err != nil {
		return 0, err
	}
	metadata, err := readImageFile(img, imageMetadataPath)
	if err != nil {
		return 0, err
	}
	var sizing struct {
		RootFSSizeMiB uint64 `yaml:"rootfsSizeMiB"`
	}
	if err := yaml.Unmarshal(metadata, &sizing); err != nil {
		return 0, fmt.Errorf("decode %s: %w", imageMetadataPath, err)
	}
	if sizing.RootFSSizeMiB == 0 {
		return 0, fmt.Errorf("%s requires a positive rootfsSizeMiB", imageMetadataPath)
	}
	return sizing.RootFSSizeMiB, nil
}

// readImageFile searches layers from newest to oldest, so reading K2's final
// metadata file normally downloads only the small layer that last modified it.
func readImageFile(img v1.Image, path string) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	for i := len(layers) - 1; i >= 0; i-- {
		data, found, err := readLayerFile(layers[i], path)
		if err != nil {
			return nil, fmt.Errorf("read layer %d: %w", i, err)
		}
		if found {
			return data, nil
		}
	}
	return nil, fmt.Errorf("%s is absent from OCI image", path)
}

func readLayerFile(layer v1.Layer, path string) (data []byte, found bool, err error) {
	stream, err := layer.Uncompressed()
	if err != nil {
		return nil, false, err
	}
	defer stream.Close()

	reader := tar.NewReader(stream)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		name := strings.TrimPrefix(strings.TrimPrefix(header.Name, "./"), "/")
		if name != path {
			continue
		}
		if !header.FileInfo().Mode().IsRegular() {
			return nil, false, fmt.Errorf("%s is not a regular file", path)
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxImageMetadataBytes+1))
		if err != nil {
			return nil, false, err
		}
		if int64(len(data)) > maxImageMetadataBytes {
			return nil, false, fmt.Errorf("%s exceeds %d bytes", path, maxImageMetadataBytes)
		}
		return data, true, nil
	}
}

// pullSinglePlatformImage resolves architecture-specific K2 tags without
// applying the operator workstation's default platform. Published tags can be
// OCI indexes because CI attaches provenance manifests; choosing by the local
// GOARCH would make an ARM node image uninspectable from an AMD64 workstation.
func pullSinglePlatformImage(ctx context.Context, ref string) (v1.Image, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, err
	}
	descriptor, err := remote.Get(parsed, remote.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if !descriptor.MediaType.IsIndex() {
		return descriptor.Image()
	}

	index, err := descriptor.ImageIndex()
	if err != nil {
		return nil, err
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}
	imageDescriptor, err := singleLinuxImageDescriptor(ref, manifest)
	if err != nil {
		return nil, err
	}
	return index.Image(imageDescriptor.Digest)
}

func singleLinuxImageDescriptor(ref string, manifest *v1.IndexManifest) (v1.Descriptor, error) {
	var imageDescriptors []v1.Descriptor
	for _, candidate := range manifest.Manifests {
		if candidate.Platform != nil && candidate.Platform.OS == "linux" && candidate.Platform.Architecture != "unknown" {
			imageDescriptors = append(imageDescriptors, candidate)
		}
	}
	if len(imageDescriptors) != 1 {
		return v1.Descriptor{}, fmt.Errorf("OCI index %s contains %d Linux image manifests; expected exactly one", ref, len(imageDescriptors))
	}
	return imageDescriptors[0], nil
}
