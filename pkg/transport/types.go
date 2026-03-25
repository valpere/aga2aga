package transport

import (
	"context"

	"github.com/valpere/aga2aga/pkg/document"
)

// Transport is the pluggable message bus abstraction. Concrete implementations
// (Redis Streams in Phase 2, Gossip P2P in Phase 5) satisfy this interface.
// All methods accept a context for cancellation and deadline propagation.
type Transport interface {
	// Publish sends doc to the named topic.
	Publish(ctx context.Context, topic string, doc *document.Document) error

	// Subscribe returns a channel that yields documents received on topic.
	Subscribe(ctx context.Context, topic string) (<-chan *document.Document, error)

	// Ack acknowledges a message by its transport-layer message ID.
	// msgID is the opaque delivery token assigned by the concrete transport
	// on receive (e.g., a Redis Streams entry ID). Callers MUST obtain msgID
	// from the transport's internal pending map keyed on document ID — never
	// from document content or Document.Extra, which is attacker-controlled.
	Ack(ctx context.Context, msgID string) error

	// Close shuts down the transport and releases resources.
	Close() error
}
