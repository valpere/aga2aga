package document

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// MaxDocumentBytes is the hard upper limit on raw Skills Document size accepted
// by Parse. It guards against memory-exhaustion attacks (e.g., YAML anchor
// amplification / "billion laughs").
//
// DO_NOT_TOUCH: raising this limit without a threat-model review enables
// memory exhaustion at the document entry point (CWE-400).
const MaxDocumentBytes = 64 * 1024 // 64 KiB

// isDelimiter reports whether line is a bare YAML front matter delimiter ("---"),
// optionally followed by \r to handle \r\n line endings.
func isDelimiter(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, "\r"), []byte("---"))
}

// findClosingDelimiter returns the byte index within content of the "\n" that
// immediately precedes a bare "---" delimiter line ("\n---" followed by \n, \r,
// or end-of-content). Returns -1 if no bare closing delimiter is found.
// Ignores occurrences like "\n---foo" that are not bare delimiters.
func findClosingDelimiter(content []byte) int {
	offset := 0
	for {
		idx := bytes.Index(content[offset:], []byte("\n---"))
		if idx == -1 {
			return -1
		}
		abs := offset + idx
		after := content[abs+4:]
		if len(after) == 0 || after[0] == '\n' || after[0] == '\r' {
			return abs
		}
		// Not a bare delimiter — advance past this occurrence and keep searching.
		offset = abs + 4
	}
}

// SplitFrontMatter splits a Skills Document into its YAML front matter bytes
// and Markdown body string. The document must begin with a bare "---" delimiter
// line (exactly "---", not "---foo"). Returns an error if no opening or closing
// bare delimiter is found.
// The returned yamlBytes include the trailing newline of the last YAML line.
func SplitFrontMatter(raw []byte) (yamlBytes []byte, body string, err error) {
	// Find and validate the opening delimiter line.
	firstNL := bytes.IndexByte(raw, '\n')
	if firstNL == -1 || !isDelimiter(raw[:firstNL]) {
		return nil, "", fmt.Errorf("SplitFrontMatter: no front matter delimiter found")
	}

	content := raw[firstNL+1:]

	closingIdx := findClosingDelimiter(content)
	if closingIdx == -1 {
		return nil, "", fmt.Errorf("SplitFrontMatter: no closing front matter delimiter found")
	}

	// yamlBytes ends with the newline that precedes "\n---".
	yamlBytes = content[:closingIdx+1]

	// Skip past "\n---" (4 bytes); then skip optional '\n' before body.
	after := content[closingIdx+4:]
	if len(after) > 0 && after[0] == '\n' {
		body = string(after[1:])
	}

	return yamlBytes, body, nil
}

// Parse splits the raw Skills Document bytes into front matter and body,
// then unmarshals the YAML into a Document. The original bytes are stored
// in doc.Raw without modification.
// Returns an error if raw exceeds MaxDocumentBytes.
func Parse(raw []byte) (*Document, error) {
	if len(raw) > MaxDocumentBytes {
		return nil, fmt.Errorf("Parse: document exceeds maximum size (%d bytes)", MaxDocumentBytes)
	}

	yamlBytes, body, err := SplitFrontMatter(raw)
	if err != nil {
		return nil, fmt.Errorf("Parse: %w", err)
	}

	var doc Document
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return nil, fmt.Errorf("Parse: unmarshal: %w", err)
	}

	doc.Body = body
	doc.Raw = make([]byte, len(raw))
	copy(doc.Raw, raw)

	return &doc, nil
}

// ParseAs parses the raw bytes into a Document and then converts the
// type-specific fields into T using As[T].
// Equivalent to Parse followed by As[T] — use this when the message type is known.
func ParseAs[T any](raw []byte) (*T, error) {
	doc, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("ParseAs: %w", err)
	}

	return As[T](doc)
}

// Serialize reconstructs the canonical Skills Document wire format from doc:
//
//	---
//	<yaml front matter>
//	---
//	<body>
//
// The YAML is produced by marshalling the Document (Envelope fields + Extra map).
// Returns an error if doc is nil or marshalling fails.
func Serialize(doc *Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("Serialize: doc is nil")
	}

	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("Serialize: marshal: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	// Ensure the closing delimiter is on its own line regardless of yaml.Marshal output.
	if len(yamlBytes) > 0 && yamlBytes[len(yamlBytes)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString("---\n")

	if doc.Body != "" {
		buf.WriteString(doc.Body)
	}

	return buf.Bytes(), nil
}
