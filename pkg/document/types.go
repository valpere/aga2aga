// Package document defines the Go struct hierarchy for all Skills Document
// message types used in the aga2aga Skills Document protocol.
package document

import (
	"fmt"

	"github.com/valpere/aga2aga/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// StringOrList is a string or list of strings, used for the `to:` envelope field.
// It marshals/unmarshals both forms transparently.
type StringOrList []string

// UnmarshalYAML handles both `to: "alice"` and `to: [alice, bob]`.
func (s *StringOrList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*s = StringOrList{value.Value}
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return fmt.Errorf("StringOrList: decode sequence: %w", err)
		}
		*s = list
	default:
		return fmt.Errorf("StringOrList: unexpected YAML node kind %v", value.Kind)
	}
	return nil
}

// Envelope holds the standard wire fields present in every Skills Document header.
type Envelope struct {
	Type      protocol.MessageType `yaml:"type"`
	Version   string               `yaml:"version"`
	ID        string               `yaml:"id,omitempty"`
	From      string               `yaml:"from,omitempty"`
	To        StringOrList         `yaml:"to,omitempty"`
	CreatedAt string               `yaml:"created_at,omitempty"`
	InReplyTo string               `yaml:"in_reply_to,omitempty"`
	ThreadID  string               `yaml:"thread_id,omitempty"`
	ExecID    string               `yaml:"exec_id,omitempty"`
	TTL       string               `yaml:"ttl,omitempty"`
	Status    string               `yaml:"status,omitempty"`
	Signature string               `yaml:"signature,omitempty"`
}

// Document is a parsed Skills Document: a structured envelope plus the
// type-specific extra fields and the raw Markdown body.
type Document struct {
	Envelope `yaml:",inline"`
	// Extra captures all YAML fields not defined in Envelope.
	// Used by As[T] to unmarshal type-specific structs.
	Extra map[string]any `yaml:",inline"`
	// Body is the Markdown content after the YAML front matter. Not marshalled.
	Body string `yaml:"-"`
	// Raw is the original unparsed bytes. Not marshalled.
	Raw []byte `yaml:"-"`
}

// As converts doc.Extra into a typed struct T via a YAML round-trip.
// The round-trip ensures that all yaml tags on T are respected.
func As[T any](doc *Document) (*T, error) {
	data, err := yaml.Marshal(doc.Extra)
	if err != nil {
		return nil, fmt.Errorf("As: marshal extra: %w", err)
	}
	var result T
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("As: unmarshal into %T: %w", result, err)
	}
	return &result, nil
}
