package document

import (
	"fmt"
	"time"

	"github.com/valpere/aga2aga/pkg/protocol"
)

// Builder constructs a Skills Document via a fluent method chain.
// Builder is NOT thread-safe — do not share across goroutines.
// Call Build() to obtain a validated *Document.
type Builder struct {
	msgType protocol.MessageType
	id      string
	from    string
	to      StringOrList
	execID  string
	body    string
	extra   map[string]any
}

// NewBuilder creates a Builder for the given message type.
// Build() auto-sets version: v1 and generated created_at.
func NewBuilder(msgType protocol.MessageType) *Builder {
	return &Builder{
		msgType: msgType,
		extra:   make(map[string]any),
	}
}

// ID sets the document id envelope field. Returns the Builder for chaining.
func (b *Builder) ID(id string) *Builder {
	b.id = id
	return b
}

// From sets the from envelope field. Returns the Builder for chaining.
func (b *Builder) From(from string) *Builder {
	b.from = from
	return b
}

// To sets one or more target recipients in the to envelope field.
// Returns the Builder for chaining.
func (b *Builder) To(targets ...string) *Builder {
	b.to = StringOrList(targets)
	return b
}

// ExecID sets the exec_id envelope field. Returns the Builder for chaining.
// Use this instead of Field("exec_id", ...) — exec_id is an envelope field
// and must be set on Envelope.ExecID for structural validation to pass.
func (b *Builder) ExecID(execID string) *Builder {
	b.execID = execID
	return b
}

// Field sets an arbitrary extra field (type-specific payload).
// Use for all fields not in the envelope (id, from, to, exec_id, body).
// Returns the Builder for chaining.
func (b *Builder) Field(key string, value any) *Builder {
	b.extra[key] = value
	return b
}

// Body sets the Markdown body content. Returns the Builder for chaining.
func (b *Builder) Body(body string) *Builder {
	b.body = body
	return b
}

// Build assembles the Document, auto-fills version: v1 and created_at (RFC3339),
// then runs full 3-layer validation via DefaultValidator.
// Returns an error if validation fails — callers cannot produce invalid documents.
func (b *Builder) Build() (*Document, error) {
	doc := &Document{
		Envelope: Envelope{
			Type:      b.msgType,
			Version:   protocol.ProtocolVersion,
			ID:        b.id,
			From:      b.from,
			To:        b.to,
			ExecID:    b.execID,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Extra: make(map[string]any, len(b.extra)),
		Body:  b.body,
	}
	for k, v := range b.extra {
		doc.Extra[k] = v
	}

	v, err := DefaultValidator()
	if err != nil {
		return nil, fmt.Errorf("Build: create validator: %w", err)
	}

	if errs := v.Validate(doc); len(errs) > 0 {
		return nil, fmt.Errorf("Build: validation failed: %v", errs[0])
	}

	return doc, nil
}

// NewGenomeBuilder returns a Builder pre-configured for agent.genome messages.
// Sets agent_id and kind in Extra. Caller must chain Field() for remaining
// required genome fields (status, identity, capabilities, tools, model_policy,
// prompt_policy, routing_policy, thresholds, constraints, mutation_policy).
func NewGenomeBuilder(agentID, kind string) *Builder {
	return NewBuilder(protocol.AgentGenome).
		Field("agent_id", agentID).
		Field("kind", kind)
}

// NewSpawnProposalBuilder returns a Builder pre-configured for agent.spawn.proposal
// messages. Sets parent_ids and candidate_id in Extra.
// Caller should chain Field("spawn_reason", ...) and ID/From.
func NewSpawnProposalBuilder(parentID, proposedID string) *Builder {
	return NewBuilder(protocol.AgentSpawnProposal).
		Field("parent_ids", []string{parentID}).
		Field("candidate_id", proposedID)
}

// NewTaskRequestBuilder returns a Builder pre-configured for task.request messages.
// Sets exec_id and from. Caller should chain ID(), To(), and Field("step", ...).
func NewTaskRequestBuilder(execID, from string) *Builder {
	return NewBuilder(protocol.TaskRequest).
		From(from).
		ExecID(execID)
}
