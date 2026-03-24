package document

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// SplitFrontMatter splits a Skills Document into its YAML front matter bytes
// and Markdown body string. The document must begin with a "---" delimiter line.
// Returns an error if no opening or closing delimiter is found.
// The returned yamlBytes include the trailing newline of the last YAML line.
func SplitFrontMatter(raw []byte) (yamlBytes []byte, body string, err error) {
	if len(raw) < 3 || !bytes.HasPrefix(raw, []byte("---")) {
		return nil, "", fmt.Errorf("SplitFrontMatter: no front matter delimiter found")
	}

	// Skip the opening "---" line.
	nl := bytes.IndexByte(raw, '\n')
	if nl == -1 {
		return nil, "", fmt.Errorf("SplitFrontMatter: no closing front matter delimiter found")
	}

	content := raw[nl+1:]

	// Find closing "---" preceded by a newline.
	closingIdx := bytes.Index(content, []byte("\n---"))
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
func Parse(raw []byte) (*Document, error) {
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
	buf.WriteString("---\n")

	if doc.Body != "" {
		buf.WriteString(doc.Body)
	}

	return buf.Bytes(), nil
}
