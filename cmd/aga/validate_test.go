package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestValidate_StrictSemanticError verifies that a document with only a semantic
// error (passes structural + schema) exits 0 without --strict and non-zero with.
func TestValidate_StrictSemanticError(t *testing.T) {
	// valid_quarantine_bad_transition.md: quarantine with from_status: proposed.
	// proposed → quarantined is not a valid transition → semantic error.
	// Structural and schema both pass.
	fixture := "../../tests/testdata/valid_quarantine_bad_transition.md"

	t.Run("without_strict_exits_ok", func(t *testing.T) {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		cmd := newRootCmd()
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"validate", fixture})
		if err := cmd.Execute(); err != nil {
			t.Errorf("Execute() without --strict = %v, want nil (semantic errors are non-fatal)", err)
		}
	})

	t.Run("with_strict_exits_error", func(t *testing.T) {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		cmd := newRootCmd()
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		cmd.SetArgs([]string{"validate", "--strict", fixture})
		err := cmd.Execute()
		if err == nil {
			t.Error("Execute() --strict = nil, want error (semantic errors are fatal with --strict)")
		}
		combined := stderr.String() + err.Error()
		if !strings.Contains(combined, "semantic") && !strings.Contains(combined, "transition") {
			t.Errorf("expected semantic/transition mention in output, got stderr=%q err=%v", stderr.String(), err)
		}
	})
}

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

func TestValidate_StrictValidFile(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"validate", "--strict", "../../tests/testdata/valid_task_request.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout = %q; want OK", stdout.String())
	}
}
