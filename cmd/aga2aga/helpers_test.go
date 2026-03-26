package main

import (
	"errors"
	"os"
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

func TestReadAndParseFile(t *testing.T) {
	// Build the oversized fixture path inside the test so we can reuse it.
	oversizedFile := func(t *testing.T) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "oversized-*.md")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		oversize := make([]byte, document.MaxDocumentBytes+1)
		for i := range oversize {
			oversize[i] = 'x'
		}
		if _, err := f.Write(oversize); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close temp file: %v", err)
		}
		return f.Name()
	}

	tests := []struct {
		name       string
		path       func(t *testing.T) string
		wantType   string
		wantErr    error
		wantNonNil bool
	}{
		{
			name:       "valid genome document",
			path:       func(*testing.T) string { return "../../tests/testdata/valid_genome.md" },
			wantType:   "agent.genome",
			wantNonNil: true,
		},
		{
			name:    "nonexistent file",
			path:    func(*testing.T) string { return "nonexistent-file.md" },
			wantErr: os.ErrNotExist,
		},
		{
			name:    "oversized file",
			path:    oversizedFile,
			wantErr: ErrDocumentTooLarge,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := readAndParseFile(tc.path(t))
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("readAndParseFile() error = nil, want %v", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("readAndParseFile() error = %v, want errors.Is(%v)", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readAndParseFile() error = %v", err)
			}
			if tc.wantNonNil && doc == nil {
				t.Fatal("readAndParseFile() returned nil doc")
			}
			if doc != nil && tc.wantType != "" && string(doc.Type) != tc.wantType {
				t.Errorf("doc.Type = %q, want %q", doc.Type, tc.wantType)
			}
		})
	}
}
