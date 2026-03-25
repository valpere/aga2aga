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
func readAndParseFile(path string) (*document.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filepath.Base(path), err)
	}
	defer f.Close()

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
