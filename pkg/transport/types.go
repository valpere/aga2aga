package transport

import (
	"context"
	"time"

	"github.com/valpere/aga2aga/pkg/document"
)

// Delivery pairs a received document with its transport-layer delivery token.
// The MsgID is opaque and assigned by the concrete transport on receive
// (e.g., a Redis Streams entry ID). It is the only authoritative source for
// calling Ack — callers MUST NOT derive it from document content or
// Document.Extra, which is attacker-controlled.
type Delivery struct {
	Doc      *document.Document
	MsgID    string    // opaque transport-layer token
	RecvedAt time.Time // wall-clock time the delivery was received
}

// Transport is the pluggable message bus abstraction. Concrete implementations
// (Redis Streams in Phase 2, Gossip P2P in Phase 5) satisfy this interface.
// All methods accept a context for cancellation and deadline propagation.
type Transport interface {
	// Publish sends doc to the named topic.
	Publish(ctx context.Context, topic string, doc *document.Document) error

	// Subscribe returns a channel that yields deliveries received on topic.
	// The channel is closed when the context is cancelled or an unrecoverable
	// error occurs. Callers must drain the channel promptly.
	Subscribe(ctx context.Context, topic string) (<-chan Delivery, error)

	// Ack acknowledges a specific message on a topic. topic and msgID must
	// come from a Delivery obtained via Subscribe — never from document
	// content or Document.Extra, which is attacker-controlled.
	Ack(ctx context.Context, topic, msgID string) error

	// Close shuts down the transport and releases resources.
	Close() error
}
