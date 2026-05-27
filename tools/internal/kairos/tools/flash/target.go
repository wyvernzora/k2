package flash

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedTarget points at a built artifact ready to flash. It carries
// the directory holding the manifest + image files, the parsed manifest,
// and the absolute paths of the compressed (`.raw.xz`) and raw (`.raw`)
// artifacts as computed from the manifest filenames.
type ResolvedTarget struct {
	Dir            string
	Manifest       Manifest
	CompressedPath string
	RawPath        string
}

// ErrNoTargets is returned when no matching rpi4cb artifact is found in
// the repo's `kairos/artifacts/` tree.
var ErrNoTargets = errors.New("flash: no rpi4cb artifact found under kairos/artifacts (build one with `earthly --allow-privileged ./kairos+image-build-artifact`)")

// ResolveRpi4cbTarget scans `<repoRoot>/kairos/artifacts/*/artifact-manifest.json`
// and returns the one matching `*rpi4cb*` (the hardware substring in the
// target identifier). Errors when zero matches exist or when multiple
// rpi4cb-shaped targets are present (would require a flavor selector,
// which the YAGNI decision deferred until a second rpi4cb target lands).
func ResolveRpi4cbTarget(repoRoot string) (ResolvedTarget, error) {
	pattern := filepath.Join(repoRoot, "kairos", "artifacts", "*", "artifact-manifest.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return ResolvedTarget{}, fmt.Errorf("glob artifacts: %w", err)
	}
	sort.Strings(matches)

	var candidates []ResolvedTarget
	for _, manifestPath := range matches {
		m, err := LoadManifest(manifestPath)
		if err != nil {
			// Skip unparseable manifests; surfacing them as hard errors
			// would block the flasher on unrelated build artifacts.
			continue
		}
		if !strings.Contains(m.Target, "rpi4cb") {
			continue
		}
		dir := filepath.Dir(manifestPath)
		candidates = append(candidates, ResolvedTarget{
			Dir:            dir,
			Manifest:       m,
			CompressedPath: filepath.Join(dir, m.Compressed.File),
			RawPath:        filepath.Join(dir, m.Raw.File),
		})
	}

	switch len(candidates) {
	case 0:
		return ResolvedTarget{}, ErrNoTargets
	case 1:
		return candidates[0], nil
	default:
		names := make([]string, 0, len(candidates))
		for _, c := range candidates {
			names = append(names, c.Manifest.Target)
		}
		return ResolvedTarget{}, fmt.Errorf(
			"flash: multiple rpi4cb artifacts found (%s); only one is supported today — delete the unused ones or wait for the --flavor flag to land",
			strings.Join(names, ", "),
		)
	}
}
