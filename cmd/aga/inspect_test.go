package main

import (
	"bytes"
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
	out := stdout.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON output, got: %s", out)
	}
}
