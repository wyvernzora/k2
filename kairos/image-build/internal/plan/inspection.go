package plan

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/kairos/image-build/internal/config"
)

type inspectionAccumulator struct {
	ociFiles    map[string]*fileInspectionAccumulator
	ociAbsent   map[string]PathInspection
	ociCommands map[string]CommandInspection
	rawFiles    map[string]map[string]*fileInspectionAccumulator
}

type fileInspectionAccumulator struct {
	Source       string
	Path         string
	Contains     []string
	containsSeen map[string]bool
	Tests        []JSONPatchOperation
	testsSeen    map[string]JSONPatchOperation
}

func newInspectionAccumulator() *inspectionAccumulator {
	return &inspectionAccumulator{
		ociFiles:    map[string]*fileInspectionAccumulator{},
		ociAbsent:   map[string]PathInspection{},
		ociCommands: map[string]CommandInspection{},
		rawFiles:    map[string]map[string]*fileInspectionAccumulator{},
	}
}

func mergeConfigInspection(parent config.Inspection, child config.Inspection) config.Inspection {
	out := parent
	out.OCI.Files = append(append([]config.FileInspection(nil), parent.OCI.Files...), child.OCI.Files...)
	out.OCI.Absent = append(append([]string(nil), parent.OCI.Absent...), child.OCI.Absent...)
	out.OCI.Commands = append(append([]string(nil), parent.OCI.Commands...), child.OCI.Commands...)
	out.Raw.Partitions = mergeRawInspectionPartitions(parent.Raw.Partitions, child.Raw.Partitions)
	return out
}

func mergeRawInspectionPartitions(parent map[string]config.RawPartitionInspection, child map[string]config.RawPartitionInspection) map[string]config.RawPartitionInspection {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}
	out := map[string]config.RawPartitionInspection{}
	for label, partition := range parent {
		out[label] = config.RawPartitionInspection{
			Files: append([]config.FileInspection(nil), partition.Files...),
		}
	}
	for label, partition := range child {
		existing := out[label]
		existing.Files = append(existing.Files, partition.Files...)
		out[label] = existing
	}
	return out
}

func (a *inspectionAccumulator) addInspection(source string, inspect config.Inspection) error {
	for _, file := range inspect.OCI.Files {
		converted, err := convertFileInspection(source, file, true)
		if err != nil {
			return err
		}
		if _, absent := a.ociAbsent[converted.Path]; absent {
			return fmt.Errorf("%s declares OCI file %s but it is also declared absent", source, converted.Path)
		}
		if err := addFileInspection(a.ociFiles, converted); err != nil {
			return err
		}
	}
	for _, path := range inspect.OCI.Absent {
		path = strings.TrimSpace(path)
		if path == "" {
			return fmt.Errorf("%s declares an empty OCI absent path", source)
		}
		if _, exists := a.ociFiles[path]; exists {
			return fmt.Errorf("%s declares OCI path %s absent but it also has file expectations", source, path)
		}
		if _, seen := a.ociAbsent[path]; !seen {
			a.ociAbsent[path] = PathInspection{Source: source, Path: path}
		}
	}
	for _, command := range inspect.OCI.Commands {
		command = strings.TrimSpace(command)
		if command == "" {
			return fmt.Errorf("%s declares an empty OCI command expectation", source)
		}
		if _, seen := a.ociCommands[command]; !seen {
			a.ociCommands[command] = CommandInspection{Source: source, Name: command}
		}
	}

	for label, partition := range inspect.Raw.Partitions {
		label = strings.TrimSpace(label)
		if label == "" {
			return fmt.Errorf("%s declares a raw inspection partition with an empty label", source)
		}
		if a.rawFiles[label] == nil {
			a.rawFiles[label] = map[string]*fileInspectionAccumulator{}
		}
		for _, file := range partition.Files {
			converted, err := convertFileInspection(source, file, false)
			if err != nil {
				return err
			}
			if filepath.IsAbs(converted.Path) || strings.HasPrefix(converted.Path, "/") {
				return fmt.Errorf("%s declares raw path %q; raw inspection paths must be partition-relative", source, converted.Path)
			}
			clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(converted.Path)))
			if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("%s declares raw path %q that escapes the partition root", source, converted.Path)
			}
			converted.Path = clean
			if err := addFileInspection(a.rawFiles[label], converted); err != nil {
				return err
			}
		}
	}

	return nil
}

func convertFileInspection(source string, file config.FileInspection, allowAbsolute bool) (FileInspection, error) {
	path := strings.TrimSpace(file.Path)
	if path == "" {
		return FileInspection{}, fmt.Errorf("%s declares a file inspection with an empty path", source)
	}
	if !allowAbsolute && strings.HasPrefix(path, "/") {
		return FileInspection{}, fmt.Errorf("%s declares raw path %q; raw inspection paths must be partition-relative", source, path)
	}

	tests := convertJSONPatchOperations(file.StructuredTests)
	if len(tests) > 0 && !isStructuredPatchTarget(path) {
		return FileInspection{}, fmt.Errorf("%s declares structured tests for unsupported file type %q", source, path)
	}
	for _, test := range tests {
		if test.Op != "test" {
			return FileInspection{}, fmt.Errorf("%s declares unsupported inspection operation %q for %s; only test is supported", source, test.Op, path)
		}
		if test.Path != "" && !strings.HasPrefix(test.Path, "/") {
			return FileInspection{}, fmt.Errorf("%s declares invalid JSON pointer %q for %s", source, test.Path, path)
		}
	}

	return FileInspection{
		Source:          source,
		Path:            path,
		Contains:        append([]string(nil), file.Contains...),
		StructuredTests: tests,
	}, nil
}

func convertJSONPatchOperations(operations []config.JSONPatchOperation) []JSONPatchOperation {
	out := make([]JSONPatchOperation, len(operations))
	for i, operation := range operations {
		out[i] = JSONPatchOperation{
			Op:    operation.Op,
			Path:  operation.Path,
			From:  operation.From,
			Value: normalizeYAMLValue(operation.Value),
		}
	}
	return out
}

func addFileInspection(files map[string]*fileInspectionAccumulator, file FileInspection) error {
	acc := files[file.Path]
	if acc == nil {
		acc = &fileInspectionAccumulator{
			Source:       file.Source,
			Path:         file.Path,
			containsSeen: map[string]bool{},
			testsSeen:    map[string]JSONPatchOperation{},
		}
		files[file.Path] = acc
	}
	for _, contains := range file.Contains {
		if acc.containsSeen[contains] {
			continue
		}
		acc.containsSeen[contains] = true
		acc.Contains = append(acc.Contains, contains)
	}
	for _, test := range file.StructuredTests {
		key := test.Op + "\x00" + test.Path
		if existing, seen := acc.testsSeen[key]; seen {
			if !reflect.DeepEqual(existing.Value, test.Value) {
				return fmt.Errorf("%s conflicts with %s for %s test %s: %#v != %#v", file.Source, acc.Source, file.Path, test.Path, test.Value, existing.Value)
			}
			continue
		}
		acc.testsSeen[key] = test
		acc.Tests = append(acc.Tests, test)
	}
	return nil
}

func (a *inspectionAccumulator) build() Inspection {
	out := Inspection{
		OCI: OCIInspection{
			Files:    buildFiles(a.ociFiles),
			Absent:   buildAbsent(a.ociAbsent),
			Commands: buildCommands(a.ociCommands),
		},
	}

	labels := make([]string, 0, len(a.rawFiles))
	for label := range a.rawFiles {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		out.Raw.Partitions = append(out.Raw.Partitions, RawPartitionInspection{
			Label: label,
			Files: buildFiles(a.rawFiles[label]),
		})
	}
	return out
}

func buildFiles(files map[string]*fileInspectionAccumulator) []FileInspection {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	out := make([]FileInspection, 0, len(paths))
	for _, path := range paths {
		file := files[path]
		out = append(out, FileInspection{
			Source:          file.Source,
			Path:            file.Path,
			Contains:        append([]string(nil), file.Contains...),
			StructuredTests: append([]JSONPatchOperation(nil), file.Tests...),
		})
	}
	return out
}

func buildAbsent(paths map[string]PathInspection) []PathInspection {
	keys := make([]string, 0, len(paths))
	for path := range paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)

	out := make([]PathInspection, 0, len(keys))
	for _, path := range keys {
		out = append(out, paths[path])
	}
	return out
}

func buildCommands(commands map[string]CommandInspection) []CommandInspection {
	keys := make([]string, 0, len(commands))
	for command := range commands {
		keys = append(keys, command)
	}
	sort.Strings(keys)

	out := make([]CommandInspection, 0, len(keys))
	for _, command := range keys {
		out = append(out, commands[command])
	}
	return out
}
