package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCreate_TaskRequest(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{
		"create", "task.request",
		"--id", "msg-test-1",
		"--from", "orchestrator",
		"--to", "reviewer",
		"--exec-id", "exec-test-1",
		"--field", "step=review",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "task.request") {
		t.Errorf("output missing type: %s", out)
	}
	if !strings.Contains(out, "msg-test-1") {
		t.Errorf("output missing id: %s", out)
	}
}

func TestCreate_UnknownType(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "unknown.type"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
}

func TestCreate_ToFile(t *testing.T) {
	outFile := t.TempDir() + "/out.md"
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "task.request",
		"--id", "msg-file-1",
		"--from", "orch",
		"--to", "agent",
		"--exec-id", "e1",
		"--field", "step=s1",
		"--out", outFile,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", outFile, err)
	}
	if !strings.Contains(string(raw), "task.request") {
		t.Errorf("file missing type: %s", string(raw))
	}
}

func TestCreate_MultipleFields(t *testing.T) {
	stdout := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetArgs([]string{
		"create", "task.request",
		"--id", "msg-mf-1",
		"--from", "orch",
		"--to", "agent",
		"--exec-id", "e1",
		"--field", "step=review",
		"--field", "priority=high",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "priority") {
		t.Errorf("output missing extra field: %s", out)
	}
}
