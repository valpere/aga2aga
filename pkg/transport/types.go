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
	Doc      *document.Document // parsed envelope document — read-only after delivery
	MsgID    string             // opaque transport-layer token; use only for Ack calls
	RecvedAt time.Time          // wall-clock receive time; for monitoring, not business logic
}

// PublishOptions configures optional parameters for a Publish call.
// Zero values apply the transport's defaults (e.g. MaxLen=0 means unlimited).
type PublishOptions struct {
	// MaxLen caps the Redis stream length (XADD MAXLEN ~). 0 = unlimited.
	MaxLen int64
}

// Transport is the pluggable message bus abstraction. Concrete implementations
// (Redis Streams in Phase 2, Gossip P2P in Phase 5) satisfy this interface.
// All methods accept a context for cancellation and deadline propagation.
type Transport interface {
	// Publish sends doc to the named topic. opts is optional; at most one
	// PublishOptions value is read (variadic for backward compatibility).
	Publish(ctx context.Context, topic string, doc *document.Document, opts ...PublishOptions) error

	// Subscribe returns a channel that yields deliveries received on topic.
	// The channel is closed when ctx is cancelled or an unrecoverable error
	// occurs (e.g., connection loss that cannot be retried). Callers must
	// drain the channel promptly to avoid blocking the transport.
	Subscribe(ctx context.Context, topic string) (<-chan Delivery, error)

	// Ack acknowledges a specific message on a topic. topic and msgID must
	// come from a Delivery obtained via Subscribe — never from document
	// content or Document.Extra, which is attacker-controlled.
	Ack(ctx context.Context, topic, msgID string) error

	// Close shuts down the transport and releases resources.
	Close() error
}
