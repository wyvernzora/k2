package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Versions struct {
	KairosVersion     string `json:"kairosVersion" yaml:"kairosVersion"`
	KairosInitVersion string `json:"kairosInitVersion" yaml:"kairosInitVersion"`
	AuroraBootVersion string `json:"aurorabootVersion" yaml:"aurorabootVersion"`
	BaseImage         string `json:"baseImage" yaml:"baseImage"`
	K3sVersion        string `json:"k3sVersion" yaml:"k3sVersion"`
	ImageRevision     string `json:"imageRevision" yaml:"imageRevision"`
	RegistryImage     string `json:"registryImage" yaml:"registryImage"`
}

type TargetsFile struct {
	Targets map[string]Target `yaml:"targets"`
}

type Target struct {
	Enabled            *bool           `yaml:"enabled,omitempty"`
	Inherits           string          `yaml:"inherits,omitempty"`
	Flavor             string          `yaml:"flavor,omitempty"`
	Role               string          `yaml:"role,omitempty"`
	Arch               string          `yaml:"arch,omitempty"`
	Hardware           string          `yaml:"hardware,omitempty"`
	KairosModel        string          `yaml:"kairosModel,omitempty"`
	Artifacts          []string        `yaml:"artifacts,omitempty"`
	Overlays           []string        `yaml:"overlays,omitempty"`
	ArtifactOptions    ArtifactOptions `yaml:"artifactOptions,omitempty"`
	Inspect            Inspection      `yaml:"inspect,omitempty"`
	artifactsSpecified bool
	overlaysSpecified  bool
}

type ArtifactOptions struct {
	Raw RawArtifactOptions `yaml:"raw,omitempty"`
}

type RawArtifactOptions struct {
	DiskStateSize *int `yaml:"diskStateSize,omitempty"`
	DiskSize      *int `yaml:"diskSize,omitempty"`
}

type OverlayMetadata struct {
	Inspect Inspection `yaml:"inspect,omitempty"`
	Build   Build      `yaml:"build,omitempty"`
}

type Build struct {
	AptPackages        []string `yaml:"aptPackages,omitempty"`
	DracutInstallItems []string `yaml:"dracutInstallItems,omitempty"`
	PostInstall        []string `yaml:"postInstall,omitempty"`
}

type Inspection struct {
	OCI OCIInspection `yaml:"oci,omitempty"`
	Raw RawInspection `yaml:"raw,omitempty"`
}

type OCIInspection struct {
	Files    []FileInspection `yaml:"files,omitempty"`
	Absent   []string         `yaml:"absent,omitempty"`
	Commands []string         `yaml:"commands,omitempty"`
}

type RawInspection struct {
	Partitions map[string]RawPartitionInspection `yaml:"partitions,omitempty"`
}

type RawPartitionInspection struct {
	Files []FileInspection `yaml:"files,omitempty"`
}

type FileInspection struct {
	Path            string               `yaml:"path"`
	Contains        []string             `yaml:"contains,omitempty"`
	StructuredTests []JSONPatchOperation `yaml:"structuredTests,omitempty"`
}

type JSONPatchOperation struct {
	Op    string `yaml:"op"`
	Path  string `yaml:"path"`
	From  string `yaml:"from,omitempty"`
	Value any    `yaml:"value,omitempty"`
}

func LoadTargets(path string) (TargetsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TargetsFile{}, err
	}

	var targets TargetsFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&targets); err != nil {
		return TargetsFile{}, err
	}
	if len(targets.Targets) == 0 {
		return TargetsFile{}, fmt.Errorf("%s does not define any targets", path)
	}

	return targets, nil
}

func LoadOverlayMetadata(path string) (OverlayMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OverlayMetadata{}, err
	}

	var metadata OverlayMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return OverlayMetadata{}, err
	}
	return metadata, nil
}

func LoadVersions(path string) (Versions, error) {
	file, err := os.Open(path)
	if err != nil {
		return Versions{}, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return Versions{}, fmt.Errorf("invalid versions line %q", line)
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if err := scanner.Err(); err != nil {
		return Versions{}, err
	}

	versions := Versions{
		KairosVersion:     values["KAIROS_VERSION"],
		KairosInitVersion: values["KAIROS_INIT_VERSION"],
		AuroraBootVersion: values["AURORABOOT_VERSION"],
		BaseImage:         values["BASE_IMAGE"],
		K3sVersion:        values["K3S_VERSION"],
		ImageRevision:     values["K2_IMAGE_REVISION"],
		RegistryImage:     values["REGISTRY_IMAGE"],
	}
	if versions.KairosVersion == "" ||
		versions.KairosInitVersion == "" ||
		versions.AuroraBootVersion == "" ||
		versions.BaseImage == "" ||
		versions.K3sVersion == "" ||
		versions.ImageRevision == "" ||
		versions.RegistryImage == "" {
		return Versions{}, fmt.Errorf("%s is missing one or more required version pins", path)
	}

	return versions, nil
}

func (t Target) ArtifactsSpecified() bool {
	return t.artifactsSpecified
}

func (t Target) OverlaysSpecified() bool {
	return t.overlaysSpecified
}

func (t *Target) UnmarshalYAML(node *yaml.Node) error {
	if err := rejectUnknownTargetKeys(node); err != nil {
		return err
	}

	type rawTarget Target
	var raw rawTarget
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*t = Target(raw)
	t.artifactsSpecified = nodeHasKey(node, "artifacts")
	t.overlaysSpecified = nodeHasKey(node, "overlays")
	return nil
}

func rejectUnknownTargetKeys(node *yaml.Node) error {
	node = unwrapDocumentNode(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	allowed := map[string]bool{
		"enabled":         true,
		"inherits":        true,
		"flavor":          true,
		"role":            true,
		"arch":            true,
		"hardware":        true,
		"kairosModel":     true,
		"artifacts":       true,
		"overlays":        true,
		"artifactOptions": true,
		"inspect":         true,
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if !allowed[key] {
			return fmt.Errorf("unknown target field %q", key)
		}
	}
	return nil
}

func nodeHasKey(node *yaml.Node, key string) bool {
	node = unwrapDocumentNode(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func unwrapDocumentNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}
