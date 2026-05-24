// Package flash drives the rpi4cb hardware flasher: rpiboot, disk
// auto-detect, write, verify, eject. macOS-only at the moment; the
// Platform interface gates anything OS-specific so a Linux flasher
// can plug in by adding a build-tagged implementation.
package flash

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manifest is the subset of `artifact-manifest.json` that the flasher
// reads. The build pipeline writes a richer document (including patch
// instructions); we ignore everything not needed here.
type Manifest struct {
	Target        string       `json:"target"`
	KairosVersion string       `json:"kairosVersion"`
	K3sVersion    string       `json:"k3sVersion"`
	Raw           ArtifactFile `json:"raw"`
	Compressed    ArtifactFile `json:"compressed"`
}

// ArtifactFile describes one image artifact (compressed or raw),
// including its filename relative to the artifact directory, its
// expected sha256 hex digest, and its size in bytes.
type ArtifactFile struct {
	File      string `json:"file"`
	SHA256    string `json:"sha256"`
	SizeBytes uint64 `json:"sizeBytes"`
}

// LoadManifest reads + parses an `artifact-manifest.json`.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if err := m.validate(path); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (m Manifest) validate(path string) error {
	if m.Target == "" {
		return fmt.Errorf("%s: target is required", path)
	}
	if m.Raw.File == "" {
		return fmt.Errorf("%s: raw.file is required", path)
	}
	if m.Raw.SHA256 == "" {
		return fmt.Errorf("%s: raw.sha256 is required", path)
	}
	if m.Raw.SizeBytes == 0 {
		return fmt.Errorf("%s: raw.sizeBytes is required", path)
	}
	if m.Compressed.File == "" {
		return fmt.Errorf("%s: compressed.file is required", path)
	}
	return nil
}
