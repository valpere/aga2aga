package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInspect_ValidFile(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "type") {
		t.Errorf("output missing 'type' field: %s", out)
	}
	if !strings.Contains(out, "task.request") {
		t.Errorf("output missing 'task.request': %s", out)
	}
}

func TestInspect_MissingFile(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"inspect", "/nonexistent/file.md"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error for missing file, got nil")
	}
}

func TestInspect_InvalidFile(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"inspect", "../../tests/testdata/invalid_missing_type.md"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error for invalid file, got nil")
	}
}

func TestInspect_JSONFormat(t *testing.T) {
	stdout := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetArgs([]string{"inspect", "--format", "json", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	if _, ok := got["type"]; !ok {
		t.Errorf("JSON output missing 'type' key: %v", got)
	}
	if _, ok := got["version"]; !ok {
		t.Errorf("JSON output missing 'version' key: %v", got)
	}
}

// TestInspect_JSONFormat_ExtraNested verifies that Extra fields (type-specific payload)
// are nested under the "extra" key in JSON output — the key security invariant that
// prevents attacker-controlled Extra keys from shadowing envelope fields.
func TestInspect_JSONFormat_ExtraNested(t *testing.T) {
	// valid_task_request.md has "step: review" in Extra (not an envelope field).
	stdout := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetArgs([]string{"inspect", "--format", "json", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}
	// Extra fields must be nested under "extra", not at the top level.
	if _, ok := got["step"]; ok {
		t.Error("Extra field 'step' is at top level — must be nested under 'extra'")
	}
	extra, ok := got["extra"]
	if !ok {
		t.Fatal("JSON output missing 'extra' key — Extra fields not nested")
	}
	extraMap, ok := extra.(map[string]any)
	if !ok {
		t.Fatalf("'extra' value is %T, want map[string]any", extra)
	}
	if step, ok := extraMap["step"]; !ok || step != "review" {
		t.Errorf("extra[step] = %v, want \"review\"", step)
	}
}

func TestInspect_UnknownFormat(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"inspect", "--format", "xml", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error for unknown format, got nil")
	}
}
