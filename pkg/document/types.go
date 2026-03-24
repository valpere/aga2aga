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

// MarshalYAML serialises a single-element StringOrList as a scalar,
// and a multi-element list as a YAML sequence.
func (s StringOrList) MarshalYAML() (any, error) {
	if len(s) == 1 {
		return s[0], nil
	}

	return []string(s), nil
}

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
//
// SECURITY: The From field is self-reported and cryptographically unverified until
// Phase 3 (Ed25519 signatures via pkg/identity). Authorization decisions MUST NOT
// be based on From alone. Use the Signature + SigningKeyID fields once pkg/identity
// is implemented.
type Envelope struct {
	Type    protocol.MessageType `yaml:"type"`
	Version string               `yaml:"version"`
	ID      string               `yaml:"id,omitempty"`
	// WARNING: unverified until Phase 3 — see struct doc above.
	From string       `yaml:"from,omitempty"`
	To   StringOrList `yaml:"to,omitempty"`
	// DO_NOT_TOUCH (for agent.genome documents): spec §5.4 — created_at is
	// immutable once set. See AgentGenome for the full immutability contract.
	CreatedAt    string `yaml:"created_at,omitempty"`
	InReplyTo    string `yaml:"in_reply_to,omitempty"`
	ThreadID     string `yaml:"thread_id,omitempty"`
	ExecID       string `yaml:"exec_id,omitempty"`
	TTL          string `yaml:"ttl,omitempty"`
	Status       string `yaml:"status,omitempty"`
	Signature    string `yaml:"signature,omitempty"`
	SigningKeyID string `yaml:"signing_key_id,omitempty"`
}

// Document is a parsed Skills Document: a structured envelope plus the
// type-specific extra fields and the raw Markdown body.
type Document struct {
	Envelope `yaml:",inline"`
	// Extra captures all YAML fields not defined in Envelope.
	// Used by As[T] to unmarshal type-specific structs.
	//
	// SECURITY: Extra MUST NOT be used directly for signing, signature
	// verification, lifecycle state transitions, or authorization decisions.
	// Always go through As[T] to obtain a typed struct.
	// Keys in Extra are unvalidated and attacker-controlled.
	Extra map[string]any `yaml:",inline"`
	// Body is the Markdown content after the YAML front matter. Not marshalled.
	Body string `yaml:"-"`
	// Raw is the original unparsed bytes. Not marshalled.
	Raw []byte `yaml:"-"`
}

// envelopeKeys is the set of yaml tag names claimed by Envelope.
// As[T] strips these from doc.Extra before the marshal round-trip so that
// attacker-controlled Extra values can never shadow Envelope fields in T.
var envelopeKeys = map[string]struct{}{ //nolint:gochecknoglobals
	"type": {}, "version": {}, "id": {}, "from": {}, "to": {},
	"created_at": {}, "in_reply_to": {}, "thread_id": {}, "exec_id": {},
	"ttl": {}, "status": {}, "signature": {}, "signing_key_id": {},
}

// As converts doc.Extra into a typed struct T via a YAML round-trip.
// The round-trip ensures that all yaml tags on T are respected.
// Envelope keys are stripped from doc.Extra before marshalling so they
// cannot bleed into T even if Extra was manipulated directly.
// Returns an error if doc is nil.
func As[T any](doc *Document) (*T, error) {
	if doc == nil {
		return nil, fmt.Errorf("As: doc is nil")
	}

	filtered := make(map[string]any, len(doc.Extra))

	for k, v := range doc.Extra {
		if _, isEnvelope := envelopeKeys[k]; !isEnvelope {
			filtered[k] = v
		}
	}

	data, err := yaml.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("As: marshal extra: %w", err)
	}

	var result T

	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("As: unmarshal into %T: %w", result, err)
	}

	return &result, nil
}
