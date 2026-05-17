package rawpatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
	"gopkg.in/yaml.v3"
)

func ApplyStructuredPatch(data []byte, targetPath string, operations []plan.JSONPatchOperation) ([]byte, error) {
	if strings.EqualFold(fileExt(targetPath), ".json") {
		return applyJSONDocumentPatch(data, operations)
	}
	return ApplyJSONPatch(data, operations)
}

func ValidateStructuredPatchResult(data []byte, targetPath string, operations []plan.JSONPatchOperation) error {
	if strings.EqualFold(fileExt(targetPath), ".json") {
		doc, err := parseJSONDocument(data)
		if err != nil {
			return err
		}
		return validatePatchResult(doc, operations)
	}
	return ValidateJSONPatchResult(data, operations)
}

func ApplyJSONPatch(data []byte, operations []plan.JSONPatchOperation) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	doc = normalizeYAMLValue(doc)

	for _, operation := range operations {
		switch operation.Op {
		case "test":
			got, err := getJSONPointer(doc, operation.Path)
			if err != nil {
				return nil, fmt.Errorf("test %s failed: %w", operation.Path, err)
			}
			if !reflect.DeepEqual(got, operation.Value) {
				return nil, fmt.Errorf("test %s failed: got %#v, want %#v", operation.Path, got, operation.Value)
			}
		case "replace":
			if err := replaceJSONPointer(doc, operation.Path, operation.Value); err != nil {
				return nil, fmt.Errorf("replace %s failed: %w", operation.Path, err)
			}
		case "add":
			var err error
			doc, err = addJSONPointer(doc, operation.Path, operation.Value)
			if err != nil {
				return nil, fmt.Errorf("add %s failed: %w", operation.Path, err)
			}
		default:
			return nil, fmt.Errorf("unsupported JSON patch operation %q", operation.Op)
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func ValidateJSONPatchResult(data []byte, operations []plan.JSONPatchOperation) error {
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	doc = normalizeYAMLValue(doc)
	return validatePatchResult(doc, operations)
}

func applyJSONDocumentPatch(data []byte, operations []plan.JSONPatchOperation) ([]byte, error) {
	doc, err := parseJSONDocument(data)
	if err != nil {
		return nil, err
	}
	for _, operation := range operations {
		switch operation.Op {
		case "test":
			got, err := getJSONPointer(doc, operation.Path)
			if err != nil {
				return nil, fmt.Errorf("test %s failed: %w", operation.Path, err)
			}
			if !reflect.DeepEqual(got, operation.Value) {
				return nil, fmt.Errorf("test %s failed: got %#v, want %#v", operation.Path, got, operation.Value)
			}
		case "replace":
			if err := replaceJSONPointer(doc, operation.Path, operation.Value); err != nil {
				return nil, fmt.Errorf("replace %s failed: %w", operation.Path, err)
			}
		case "add":
			var err error
			doc, err = addJSONPointer(doc, operation.Path, operation.Value)
			if err != nil {
				return nil, fmt.Errorf("add %s failed: %w", operation.Path, err)
			}
		default:
			return nil, fmt.Errorf("unsupported JSON patch operation %q", operation.Op)
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func parseJSONDocument(data []byte) (any, error) {
	var doc any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	return normalizeJSONValue(doc), nil
}

func validatePatchResult(doc any, operations []plan.JSONPatchOperation) error {
	for _, operation := range operations {
		switch operation.Op {
		case "test", "replace", "add":
			got, err := getJSONPointer(doc, operation.Path)
			if err != nil {
				return fmt.Errorf("%s %s validation failed: %w", operation.Op, operation.Path, err)
			}
			if !reflect.DeepEqual(got, operation.Value) {
				return fmt.Errorf("%s %s validation failed: got %#v, want %#v", operation.Op, operation.Path, got, operation.Value)
			}
		default:
			return fmt.Errorf("unsupported JSON patch operation %q", operation.Op)
		}
	}
	return nil
}

func addJSONPointer(doc any, pointer string, value any) (any, error) {
	segments, err := pointerSegments(pointer)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return value, nil
	}
	return addAtJSONPointer(doc, segments, value)
}

func addAtJSONPointer(current any, segments []string, value any) (any, error) {
	segment := segments[0]
	if len(segments) == 1 {
		switch typed := current.(type) {
		case map[string]any:
			typed[segment] = value
			return typed, nil
		case []any:
			index, err := parseAddIndex(segment, len(typed))
			if err != nil {
				return nil, err
			}
			typed = append(typed, nil)
			copy(typed[index+1:], typed[index:])
			typed[index] = value
			return typed, nil
		default:
			return nil, fmt.Errorf("cannot add child %q on %T", segment, current)
		}
	}

	switch typed := current.(type) {
	case map[string]any:
		child, ok := typed[segment]
		if !ok {
			return nil, fmt.Errorf("missing object key %q", segment)
		}
		child, err := addAtJSONPointer(child, segments[1:], value)
		if err != nil {
			return nil, err
		}
		typed[segment] = child
		return typed, nil
	case []any:
		index, err := parseIndex(segment, len(typed))
		if err != nil {
			return nil, err
		}
		child, err := addAtJSONPointer(typed[index], segments[1:], value)
		if err != nil {
			return nil, err
		}
		typed[index] = child
		return typed, nil
	default:
		return nil, fmt.Errorf("cannot read child %q on %T", segment, current)
	}
}

func getJSONPointer(doc any, pointer string) (any, error) {
	if pointer == "" {
		return doc, nil
	}

	segments, err := pointerSegments(pointer)
	if err != nil {
		return nil, err
	}
	current := doc
	for _, segment := range segments {
		next, err := child(current, segment)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func replaceJSONPointer(doc any, pointer string, value any) error {
	segments, err := pointerSegments(pointer)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return fmt.Errorf("replacing the document root is not supported")
	}

	parent := doc
	for _, segment := range segments[:len(segments)-1] {
		next, err := child(parent, segment)
		if err != nil {
			return err
		}
		parent = next
	}

	last := segments[len(segments)-1]
	switch typed := parent.(type) {
	case map[string]any:
		if _, ok := typed[last]; !ok {
			return fmt.Errorf("missing object key %q", last)
		}
		typed[last] = value
	case []any:
		index, err := parseIndex(last, len(typed))
		if err != nil {
			return err
		}
		typed[index] = value
	default:
		return fmt.Errorf("cannot replace child %q on %T", last, parent)
	}
	return nil
}

func child(value any, segment string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		child, ok := typed[segment]
		if !ok {
			return nil, fmt.Errorf("missing object key %q", segment)
		}
		return child, nil
	case []any:
		index, err := parseIndex(segment, len(typed))
		if err != nil {
			return nil, err
		}
		return typed[index], nil
	default:
		return nil, fmt.Errorf("cannot read child %q on %T", segment, value)
	}
}

func pointerSegments(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON pointer %q must start with /", pointer)
	}
	raw := strings.Split(pointer[1:], "/")
	segments := make([]string, len(raw))
	for i, segment := range raw {
		segments[i] = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
	}
	return segments, nil
}

func parseIndex(segment string, length int) (int, error) {
	index, err := strconv.Atoi(segment)
	if err != nil {
		return 0, fmt.Errorf("array index %q is not numeric", segment)
	}
	if index < 0 || index >= length {
		return 0, fmt.Errorf("array index %d is out of range", index)
	}
	return index, nil
}

func parseAddIndex(segment string, length int) (int, error) {
	if segment == "-" {
		return length, nil
	}
	index, err := strconv.Atoi(segment)
	if err != nil {
		return 0, fmt.Errorf("array index %q is not numeric", segment)
	}
	if index < 0 || index > length {
		return 0, fmt.Errorf("array index %d is out of range", index)
	}
	return index, nil
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

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range typed {
			out[key] = normalizeJSONValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeJSONValue(value)
		}
		return out
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return int(i)
		}
		if f, err := typed.Float64(); err == nil {
			return f
		}
		return string(typed)
	default:
		return value
	}
}

func fileExt(path string) string {
	index := strings.LastIndex(path, ".")
	if index == -1 {
		return ""
	}
	return path[index:]
}
