package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRoot_Version(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "v1") {
		t.Errorf("version output = %q; want to contain protocol version v1", buf.String())
	}
}

func TestRoot_NoArgs(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{})
	// Root with no args should not error — just prints help.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() with no args error = %v", err)
	}
}
