package toolcli

import (
	"reflect"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/workflow"
)

func TestRegistrationTagsPreserveWorkflowAliases(t *testing.T) {
	registration := workflow.Registration{Aliases: []string{"first", "second"}}
	want := []string{`aliases:"first,second"`}
	if got := registrationTags(registration); !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
}

func TestRegistrationTagsOmitEmptyAliases(t *testing.T) {
	if got := registrationTags(workflow.Registration{}); got != nil {
		t.Fatalf("tags = %#v, want nil", got)
	}
}

func TestBuiltinRegistrationsExposeStableCommandSurface(t *testing.T) {
	registrations := builtinRegistrations()
	want := []string{"provision", "e2e", "vm", "flash", "upgrade", "image", "build"}
	if len(registrations) != len(want) {
		t.Fatalf("registrations = %d, want %d", len(registrations), len(want))
	}

	seen := make(map[string]struct{}, len(registrations))
	for i, registration := range registrations {
		if registration.Name != want[i] {
			t.Fatalf("registration %d name = %q, want %q", i, registration.Name, want[i])
		}
		if registration.Help == "" || registration.Command == nil {
			t.Fatalf("registration %q is incomplete", registration.Name)
		}
		if _, ok := seen[registration.Name]; ok {
			t.Fatalf("duplicate registration %q", registration.Name)
		}
		seen[registration.Name] = struct{}{}
	}
}

func TestBuiltinRegistrationsReturnFreshCommandSchemas(t *testing.T) {
	first := builtinRegistrations()
	second := builtinRegistrations()
	for i := range first {
		if reflect.ValueOf(first[i].Command).Pointer() == reflect.ValueOf(second[i].Command).Pointer() {
			t.Fatalf("registration %q reused mutable command schema", first[i].Name)
		}
	}
}
