package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/valpere/aga2aga/pkg/document"
)

// ErrDocumentTooLarge is returned when the file exceeds MaxDocumentBytes.
var ErrDocumentTooLarge = errors.New("document exceeds maximum size")

// readAndParseFile opens path, enforces the MaxDocumentBytes limit, and
// returns the parsed Document. Returns a descriptive error on open, read,
// size, or parse failure.
//
// SECURITY: path MUST be a value supplied directly by the local operator
// (e.g., a CLI argument). It MUST NOT be derived from document content,
// agent-controlled wire data, or any remote input. Callers in daemon or
// MCP-handler contexts must perform filepath.EvalSymlinks and root-confinement
// checks before calling this function. (CWE-22, CWE-61)
func readAndParseFile(path string) (*document.Document, error) {
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filepath.Base(path), err)
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filepath.Base(path), err)
	}
	defer f.Close()

	// LimitReader cap is MaxDocumentBytes+1 so that a file of exactly
	// MaxDocumentBytes passes (len(raw) == MaxDocumentBytes, not >),
	// while any larger file produces len(raw) == MaxDocumentBytes+1 and
	// is caught by the check below. DO NOT reduce the cap to MaxDocumentBytes.
	raw, err := io.ReadAll(io.LimitReader(f, document.MaxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", filepath.Base(path), err)
	}
	if len(raw) > document.MaxDocumentBytes {
		return nil, fmt.Errorf("%w (%d bytes)", ErrDocumentTooLarge, document.MaxDocumentBytes)
	}

	doc, err := document.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", filepath.Base(path), err)
	}
	return doc, nil
}
