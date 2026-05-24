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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Image is the per-tag fact set k2-tools upgrade actually wires into
// its Plan section: the fully qualified OCI ref, the resolved digest
// (so the operator pins by content even when calling kairos-agent
// upgrade with the tag), and the registry-side creation time used
// for the "published N days ago" line.
type Image struct {
	Ref     string    // e.g. ghcr.io/wyvernzora/k2-kairos:ubuntu-...-rev3
	Digest  string    // sha256:abc...
	Created time.Time // OCI image config `created` field
}

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
	return Image{
		Ref:     ref,
		Digest:  digest,
		Created: cfg.Created.Time,
	}, nil
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
// All methods take a context for cancellation hygiene; crane itself
// is context-free for these calls, but we honor ctx.Err() before each
// network round-trip via the wrapper below.
type registry interface {
	ListTags(ctx context.Context, repo string) ([]string, error)
	Config(ctx context.Context, ref string) (*v1.ConfigFile, error)
	Digest(ctx context.Context, ref string) (string, error)
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	img, err := crane.Pull(ref)
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
