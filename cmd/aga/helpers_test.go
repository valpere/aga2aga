package main

import (
	"os"
	"strings"
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

func TestReadAndParseFile_ValidFile(t *testing.T) {
	doc, err := readAndParseFile("../../tests/testdata/valid_genome.md")
	if err != nil {
		t.Fatalf("readAndParseFile() error = %v", err)
	}
	if doc == nil {
		t.Fatal("readAndParseFile() returned nil doc")
	}
	if doc.Type != "agent.genome" {
		t.Errorf("doc.Type = %q, want agent.genome", doc.Type)
	}
}

func TestReadAndParseFile_NonexistentFile(t *testing.T) {
	_, err := readAndParseFile("nonexistent-file.md")
	if err == nil {
		t.Fatal("readAndParseFile() expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error = %q, want 'open' in message", err.Error())
	}
}

func TestReadAndParseFile_OversizedFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "oversized-*.md")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	// Write content larger than MaxDocumentBytes
	oversize := make([]byte, document.MaxDocumentBytes+1)
	for i := range oversize {
		oversize[i] = 'x'
	}
	if _, err := f.Write(oversize); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()

	_, err = readAndParseFile(f.Name())
	if err == nil {
		t.Fatal("readAndParseFile() expected error for oversized file, got nil")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Errorf("error = %q, want 'maximum size' in message", err.Error())
	}
}
