package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/kairos/image-build/internal/config"
	"github.com/wyvernzora/k2/kairos/image-build/internal/paths"
	"gopkg.in/yaml.v3"
)

type Planner struct {
	Targets  config.TargetsFile
	Versions config.Versions
	Paths    paths.Paths
}

type Plan struct {
	Target           string          `json:"target" yaml:"target"`
	Enabled          bool            `json:"enabled" yaml:"enabled"`
	Flavor           string          `json:"flavor" yaml:"flavor"`
	FlavorRelease    string          `json:"flavorRelease" yaml:"flavorRelease"`
	Variant          string          `json:"variant" yaml:"variant"`
	Arch             string          `json:"arch" yaml:"arch"`
	Platform         string          `json:"platform" yaml:"platform"`
	Hardware         string          `json:"hardware" yaml:"hardware"`
	KairosModel      string          `json:"kairosModel" yaml:"kairosModel"`
	Role             string          `json:"role" yaml:"role"`
	KubernetesDistro string          `json:"kubernetesDistro" yaml:"kubernetesDistro"`
	Artifacts        []string        `json:"artifacts" yaml:"artifacts"`
	Overlays         []string        `json:"overlays" yaml:"overlays"`
	RawPatches       []RawPatch      `json:"rawPatches" yaml:"rawPatches"`
	Inspection       Inspection      `json:"inspection,omitempty" yaml:"inspection,omitempty"`
	ArtifactOptions  ArtifactOptions `json:"artifactOptions" yaml:"artifactOptions"`
	Image            string          `json:"image" yaml:"image"`
	ArtifactStem     string          `json:"artifactStem" yaml:"artifactStem"`
	ArtifactDir      string          `json:"artifactDir" yaml:"artifactDir"`
	Versions         config.Versions `json:"versions" yaml:"versions"`
	Paths            PlanPaths       `json:"paths" yaml:"paths"`
}

type PlanPaths struct {
	BuildRoot    string `json:"buildRoot" yaml:"buildRoot"`
	KairosRoot   string `json:"kairosRoot" yaml:"kairosRoot"`
	TargetsFile  string `json:"targetsFile" yaml:"targetsFile"`
	VersionsFile string `json:"versionsFile" yaml:"versionsFile"`
	OverlaysDir  string `json:"overlaysDir" yaml:"overlaysDir"`
	ArtifactsDir string `json:"artifactsDir" yaml:"artifactsDir"`
}

type ArtifactOptions struct {
	Raw RawArtifactOptions `json:"raw,omitempty" yaml:"raw,omitempty"`
}

type RawArtifactOptions struct {
	DiskStateSize *int `json:"diskStateSize,omitempty" yaml:"diskStateSize,omitempty"`
}

type RawPatch struct {
	Type           string               `json:"type" yaml:"type"`
	Overlay        string               `json:"overlay" yaml:"overlay"`
	Source         string               `json:"source" yaml:"source"`
	PartitionLabel string               `json:"partitionLabel" yaml:"partitionLabel"`
	Path           string               `json:"path" yaml:"path"`
	TargetPath     string               `json:"targetPath,omitempty" yaml:"targetPath,omitempty"`
	Operations     []JSONPatchOperation `json:"operations,omitempty" yaml:"operations,omitempty"`
}

type Inspection struct {
	OCI OCIInspection `json:"oci,omitempty" yaml:"oci,omitempty"`
	Raw RawInspection `json:"raw,omitempty" yaml:"raw,omitempty"`
}

type OCIInspection struct {
	Files    []FileInspection    `json:"files,omitempty" yaml:"files,omitempty"`
	Absent   []PathInspection    `json:"absent,omitempty" yaml:"absent,omitempty"`
	Commands []CommandInspection `json:"commands,omitempty" yaml:"commands,omitempty"`
}

type RawInspection struct {
	Partitions []RawPartitionInspection `json:"partitions,omitempty" yaml:"partitions,omitempty"`
}

type RawPartitionInspection struct {
	Label string           `json:"label" yaml:"label"`
	Files []FileInspection `json:"files,omitempty" yaml:"files,omitempty"`
}

type FileInspection struct {
	Source          string               `json:"source" yaml:"source"`
	Path            string               `json:"path" yaml:"path"`
	Contains        []string             `json:"contains,omitempty" yaml:"contains,omitempty"`
	StructuredTests []JSONPatchOperation `json:"structuredTests,omitempty" yaml:"structuredTests,omitempty"`
}

type PathInspection struct {
	Source string `json:"source" yaml:"source"`
	Path   string `json:"path" yaml:"path"`
}

type CommandInspection struct {
	Source string `json:"source" yaml:"source"`
	Name   string `json:"name" yaml:"name"`
}

type JSONPatchOperation struct {
	Op    string `json:"op" yaml:"op"`
	Path  string `json:"path" yaml:"path"`
	From  string `json:"from,omitempty" yaml:"from,omitempty"`
	Value any    `json:"value,omitempty" yaml:"value,omitempty"`
}

func New(targets config.TargetsFile, versions config.Versions, discovered paths.Paths) Planner {
	return Planner{
		Targets:  targets,
		Versions: versions,
		Paths:    discovered,
	}
}

func (p Planner) EnabledTargets() []string {
	var names []string
	for name, target := range p.Targets.Targets {
		if target.Enabled != nil && *target.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (p Planner) BuildAllEnabled() ([]Plan, error) {
	targets := p.EnabledTargets()
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		resolved, err := p.Build(target)
		if err != nil {
			return nil, err
		}
		plans = append(plans, resolved)
	}
	return plans, nil
}

func (p Planner) Build(target string) (Plan, error) {
	resolved, err := p.resolveTarget(target, map[string]bool{})
	if err != nil {
		return Plan{}, err
	}

	image := p.imageTag(resolved)
	artifactStem := image[strings.LastIndex(image, ":")+1:]
	out := Plan{
		Target:           target,
		Enabled:          boolValue(resolved.Enabled),
		Flavor:           resolved.Flavor,
		FlavorRelease:    resolved.FlavorRelease,
		Variant:          resolved.Variant,
		Arch:             resolved.Arch,
		Platform:         resolved.Platform,
		Hardware:         resolved.Hardware,
		KairosModel:      resolved.KairosModel,
		Role:             resolved.Role,
		KubernetesDistro: resolved.KubernetesDistro,
		Artifacts:        append([]string(nil), resolved.Artifacts...),
		Overlays:         append([]string(nil), resolved.Overlays...),
		ArtifactOptions:  convertArtifactOptions(resolved.ArtifactOptions),
		Image:            image,
		ArtifactStem:     artifactStem,
		ArtifactDir:      filepath.Join(p.Paths.ArtifactsDir, target),
		Versions:         p.Versions,
		Paths:            p.planPaths(),
	}

	rawPatches, err := p.rawPatches(resolved.Overlays)
	if err != nil {
		return Plan{}, err
	}
	out.RawPatches = rawPatches
	inspection, err := p.inspection(target, resolved)
	if err != nil {
		return Plan{}, err
	}
	out.Inspection = inspection

	if err := p.validate(out); err != nil {
		return Plan{}, err
	}
	return out, nil
}

func (p Planner) resolveTarget(name string, seen map[string]bool) (config.Target, error) {
	target, ok := p.Targets.Targets[name]
	if !ok {
		return config.Target{}, fmt.Errorf("unknown target %q", name)
	}
	if seen[name] {
		return config.Target{}, fmt.Errorf("target inheritance cycle at %q", name)
	}
	seen[name] = true

	if target.Inherits == "" {
		return target, nil
	}

	parent, err := p.resolveTarget(target.Inherits, seen)
	if err != nil {
		return config.Target{}, err
	}

	merged := parent
	merged.Inherits = target.Inherits
	mergeString(&merged.Flavor, target.Flavor)
	mergeString(&merged.FlavorRelease, target.FlavorRelease)
	mergeString(&merged.Variant, target.Variant)
	mergeString(&merged.Arch, target.Arch)
	mergeString(&merged.Platform, target.Platform)
	mergeString(&merged.Hardware, target.Hardware)
	mergeString(&merged.KairosModel, target.KairosModel)
	mergeString(&merged.Role, target.Role)
	mergeString(&merged.KubernetesDistro, target.KubernetesDistro)
	if target.Enabled != nil {
		merged.Enabled = target.Enabled
	}
	if target.ArtifactsSpecified() {
		merged.Artifacts = append([]string(nil), target.Artifacts...)
	}
	if target.OverlaysSpecified() {
		merged.Overlays = dedupe(append(append([]string(nil), parent.Overlays...), target.Overlays...))
	}
	if target.ArtifactOptions.Raw.DiskStateSize != nil {
		merged.ArtifactOptions.Raw.DiskStateSize = target.ArtifactOptions.Raw.DiskStateSize
	}
	merged.Inspect = mergeConfigInspection(parent.Inspect, target.Inspect)
	return merged, nil
}

func (p Planner) validate(resolved Plan) error {
	if resolved.Flavor == "" ||
		resolved.FlavorRelease == "" ||
		resolved.Variant == "" ||
		resolved.Arch == "" ||
		resolved.Platform == "" ||
		resolved.Hardware == "" ||
		resolved.KairosModel == "" ||
		resolved.Role == "" ||
		resolved.KubernetesDistro == "" {
		return fmt.Errorf("target %q is missing one or more required fields", resolved.Target)
	}

	expectedPlatform := map[string]string{
		"amd64": "linux/amd64",
		"arm64": "linux/arm64",
	}[resolved.Arch]
	if expectedPlatform == "" {
		return fmt.Errorf("target %q has unsupported arch %q", resolved.Target, resolved.Arch)
	}
	if resolved.Platform != expectedPlatform {
		return fmt.Errorf("target %q arch/platform mismatch: %s expects %s, got %s", resolved.Target, resolved.Arch, expectedPlatform, resolved.Platform)
	}

	for _, artifact := range resolved.Artifacts {
		if artifact != "raw" && artifact != "iso" {
			return fmt.Errorf("target %q has unsupported artifact type %q", resolved.Target, artifact)
		}
	}
	if len(resolved.Artifacts) == 0 {
		return fmt.Errorf("target %q must declare at least one artifact type", resolved.Target)
	}

	hasRaw := contains(resolved.Artifacts, "raw")
	if hasRaw {
		if resolved.ArtifactOptions.Raw.DiskStateSize == nil {
			return fmt.Errorf("target %q raw artifact requires diskStateSize", resolved.Target)
		}
	}

	for _, overlay := range resolved.Overlays {
		info, err := os.Stat(filepath.Join(p.Paths.OverlaysDir, overlay))
		if err != nil {
			return fmt.Errorf("target %q overlay %q is not available under %s", resolved.Target, overlay, p.Paths.OverlaysDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("target %q overlay %q is not a directory", resolved.Target, overlay)
		}
	}

	return nil
}

func (p Planner) inspection(targetName string, resolved config.Target) (Inspection, error) {
	acc := newInspectionAccumulator()
	if err := acc.addInspection("target:"+targetName, resolved.Inspect); err != nil {
		return Inspection{}, err
	}

	for _, overlay := range resolved.Overlays {
		metadataPath := filepath.Join(p.Paths.OverlaysDir, overlay, "overlay.yaml")
		if _, err := os.Stat(metadataPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Inspection{}, err
		}
		metadata, err := config.LoadOverlayMetadata(metadataPath)
		if err != nil {
			return Inspection{}, fmt.Errorf("load overlay metadata %s: %w", metadataPath, err)
		}
		if err := acc.addInspection("overlay:"+overlay, metadata.Inspect); err != nil {
			return Inspection{}, err
		}
	}

	return acc.build(), nil
}

func (p Planner) imageTag(target config.Target) string {
	k3sTag := strings.ReplaceAll(p.Versions.K3sVersion, "+", "-")
	return fmt.Sprintf(
		"%s:%s-%s-%s-%s-%s-%s-%s-%s-%s-%s",
		p.Versions.RegistryImage,
		target.Flavor,
		target.FlavorRelease,
		target.Variant,
		p.Versions.KairosVersion,
		target.Arch,
		target.Hardware,
		target.KubernetesDistro,
		k3sTag,
		target.Role,
		p.Versions.ImageRevision,
	)
}

func (p Planner) rawPatches(overlays []string) ([]RawPatch, error) {
	var result []RawPatch
	for _, overlay := range overlays {
		rawDir := filepath.Join(p.Paths.OverlaysDir, overlay, "raw")
		if !dirHasContent(rawDir) {
			continue
		}
		patches, err := p.rawPatchesForOverlay(overlay, rawDir)
		if err != nil {
			return nil, err
		}
		result = append(result, patches...)
	}
	return result, nil
}

func (p Planner) rawPatchesForOverlay(overlay string, rawDir string) ([]RawPatch, error) {
	var result []RawPatch
	if err := filepath.WalkDir(rawDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeType != 0 {
			return nil
		}

		rel, err := filepath.Rel(rawDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("raw patch file %s must be under raw/<PARTITION_LABEL>/...", path)
		}

		partitionLabel := parts[0]
		partitionPath := parts[1]
		source := filepath.ToSlash(filepath.Join("raw", rel))
		if strings.HasSuffix(partitionPath, ".patch") {
			targetPath := strings.TrimSuffix(partitionPath, ".patch")
			if !isStructuredPatchTarget(targetPath) {
				return fmt.Errorf("raw patch %s targets unsupported file type %q", source, targetPath)
			}
			operations, err := loadJSONPatchOperations(path)
			if err != nil {
				return err
			}
			result = append(result, RawPatch{
				Type:           "yaml-json-patch",
				Overlay:        overlay,
				Source:         source,
				PartitionLabel: partitionLabel,
				Path:           partitionPath,
				TargetPath:     targetPath,
				Operations:     operations,
			})
			return nil
		}

		result = append(result, RawPatch{
			Type:           "copy-to-partition",
			Overlay:        overlay,
			Source:         source,
			PartitionLabel: partitionLabel,
			Path:           partitionPath,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].PartitionLabel != result[j].PartitionLabel {
			return result[i].PartitionLabel < result[j].PartitionLabel
		}
		return result[i].Path < result[j].Path
	})
	return result, nil
}

func isStructuredPatchTarget(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func loadJSONPatchOperations(path string) ([]JSONPatchOperation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var operations []JSONPatchOperation
	if err := yaml.Unmarshal(data, &operations); err != nil {
		return nil, fmt.Errorf("parse JSON patch operations from %s: %w", path, err)
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("raw patch %s does not contain any operations", path)
	}
	for i := range operations {
		operations[i].Value = normalizeYAMLValue(operations[i].Value)
	}
	return operations, nil
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[any]any:
		out := map[string]any{}
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeYAMLValue(value)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, value := range typed {
			out[key] = normalizeYAMLValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeYAMLValue(value)
		}
		return out
	default:
		return value
	}
}

func (p Planner) planPaths() PlanPaths {
	return PlanPaths{
		BuildRoot:    p.Paths.BuildRoot,
		KairosRoot:   p.Paths.KairosRoot,
		TargetsFile:  p.Paths.TargetsFile,
		VersionsFile: p.Paths.VersionsFile,
		OverlaysDir:  p.Paths.OverlaysDir,
		ArtifactsDir: p.Paths.ArtifactsDir,
	}
}

func convertArtifactOptions(options config.ArtifactOptions) ArtifactOptions {
	return ArtifactOptions{
		Raw: RawArtifactOptions{
			DiskStateSize: options.Raw.DiskStateSize,
		},
	}
}

func mergeString(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func dirHasContent(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != ".gitkeep" {
			return true
		}
	}
	return false
}
