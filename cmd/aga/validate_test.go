package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidate_ValidFile(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout = %q; want OK", stdout.String())
	}
}

func TestValidate_InvalidFile(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", "../../tests/testdata/invalid_missing_type.md"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for invalid file, got nil")
	}
	combined := stderr.String() + err.Error()
	if !strings.Contains(combined, "structural") {
		t.Errorf("expected structural error mention, got stderr=%q err=%v", stderr.String(), err)
	}
}

func TestValidate_MissingFile(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", "/nonexistent/file.md"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error for missing file, got nil")
	}
}
