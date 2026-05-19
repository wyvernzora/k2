package prereq

import (
	"strings"
	"testing"
)

func TestRequireReportsMissingCommands(t *testing.T) {
	err := Require("definitely-not-a-k2-command")
	if err == nil {
		t.Fatal("expected missing command error")
	}
	if !strings.Contains(err.Error(), "definitely-not-a-k2-command") {
		t.Fatalf("error = %v", err)
	}
}
