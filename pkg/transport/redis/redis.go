package redistransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/transport"
)

// DefaultConsumerGroup and DefaultConsumer are exported so callers and tests
// can reference the default configuration values without hardcoding strings.
const (
	DefaultConsumerGroup = "gateway-group"
	DefaultConsumer      = "mcp-gateway"
)

// Options configures a RedisTransport.
type Options struct {
	// ConsumerGroup is the Redis consumer group name. Default: DefaultConsumerGroup.
	ConsumerGroup string
	// Consumer is the consumer name within the group. Default: DefaultConsumer.
	Consumer string
	// BlockTimeout is the XREADGROUP block duration. Default: 5s.
	// Use a short value (e.g. 100ms) in tests to keep them fast.
	BlockTimeout time.Duration
	// Logger receives operational log lines (malformed entries, transient errors).
	// Defaults to a discard logger so library callers control log output.
	Logger *log.Logger
}

func (o *Options) applyDefaults() {
	if o.ConsumerGroup == "" {
		o.ConsumerGroup = DefaultConsumerGroup
	}
	if o.Consumer == "" {
		o.Consumer = DefaultConsumer
	}
	if o.BlockTimeout == 0 {
		o.BlockTimeout = 5 * time.Second
	}
	if o.Logger == nil {
		o.Logger = log.New(io.Discard, "", 0)
	}
}

// RedisTransport implements transport.Transport using Redis Streams.
type RedisTransport struct {
	client *goredis.Client
	opts   Options
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a RedisTransport with the given Redis client and options.
// The caller retains ownership of client and must close it separately.
// Close is safe to call multiple times.
func New(client *goredis.Client, opts Options) *RedisTransport {
	opts.applyDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisTransport{client: client, opts: opts, ctx: ctx, cancel: cancel}
}

// Publish serializes doc and writes it to the named Redis stream topic via XADD.
//
// SECURITY: topic must be a statically known stream name. It MUST NOT be
// derived from doc.Extra or any other attacker-controlled document field.
func (rt *RedisTransport) Publish(ctx context.Context, topic string, doc *document.Document, opts ...transport.PublishOptions) error {
	raw, err := document.Serialize(doc)
	if err != nil {
		return fmt.Errorf("transport/redis: serialize: %w", err)
	}
	args := &goredis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"doc": string(raw)},
	}
	if len(opts) > 0 && opts[0].MaxLen > 0 {
		args.MaxLen = opts[0].MaxLen
		args.Approx = true
	}
	if err := rt.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("transport/redis: xadd on %q: %w", topic, err)
	}
	return nil
}

// Subscribe creates a consumer group (idempotent) and starts a background
// XREADGROUP loop that sends Delivery values on the returned channel.
// The channel is closed when ctx is cancelled or Close() is called.
//
// SECURITY: topic must be a statically known stream name. It MUST NOT be
// derived from document content or Document.Extra, which is attacker-controlled.
func (rt *RedisTransport) Subscribe(ctx context.Context, topic string) (<-chan transport.Delivery, error) {
	// XGROUP CREATE MKSTREAM "$": start from the latest entry, create the
	// stream if it doesn't exist. Ignore BUSYGROUP (group already exists).
	err := rt.client.XGroupCreateMkStream(ctx, topic, rt.opts.ConsumerGroup, "$").Err()
	if err != nil && !isBusyGroupErr(err) {
		return nil, fmt.Errorf("transport/redis: create consumer group on %q: %w", topic, err)
	}
	ch := make(chan transport.Delivery, 1)
	go rt.readLoop(ctx, topic, ch)
	return ch, nil
}

// Ack acknowledges msgID on topic via XACK, removing it from the PEL.
// topic and msgID must come from a Delivery obtained via Subscribe — never
// from document content or Document.Extra, which is attacker-controlled.
//
// SECURITY: topic must be a statically known stream name. It MUST NOT be
// derived from document content or Document.Extra.
func (rt *RedisTransport) Ack(ctx context.Context, topic, msgID string) error {
	if err := rt.client.XAck(ctx, topic, rt.opts.ConsumerGroup, msgID).Err(); err != nil {
		return fmt.Errorf("transport/redis: xack on %q msgID=%q: %w", topic, msgID, err)
	}
	return nil
}

// Close cancels the internal context, which stops all active subscribe loops
// and closes their channels. It does not close the underlying Redis client.
// Safe to call multiple times.
func (rt *RedisTransport) Close() error {
	rt.cancel()
	return nil
}

// readLoop runs the XREADGROUP polling loop for a single subscription.
func (rt *RedisTransport) readLoop(ctx context.Context, topic string, ch chan<- transport.Delivery) {
	defer close(ch)
	for {
		// Check cancellation before issuing a blocking call.
		select {
		case <-ctx.Done():
			return
		case <-rt.ctx.Done():
			return
		default:
		}

		msgs, err := rt.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    rt.opts.ConsumerGroup,
			Consumer: rt.opts.Consumer,
			Streams:  []string{topic, ">"},
			Count:    10,
			Block:    rt.opts.BlockTimeout,
		}).Result()
		if err != nil {
			if isCtxErr(err) || isNetClosedErr(err) {
				return
			}
			if errors.Is(err, goredis.Nil) {
				// Block timeout — no new messages; loop and re-check ctx.
				continue
			}
			rt.opts.Logger.Printf("transport/redis: XReadGroup on %q: %v", topic, err)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				d, ok := rt.parseMsg(msg, topic)
				if !ok {
					// ACK malformed entries to clear them from the PEL and
					// prevent unbounded PEL growth (CWE-400). The entry is
					// logged and discarded; a dead-letter stream should be
					// used in production for forensic analysis.
					_ = rt.client.XAck(ctx, topic, rt.opts.ConsumerGroup, msg.ID)
					continue
				}
				select {
				case ch <- d:
				case <-ctx.Done():
					return
				case <-rt.ctx.Done():
					return
				}
			}
		}
	}
}

// parseMsg extracts a Delivery from a Redis stream message.
// Returns false and logs if the entry is malformed or unparseable.
// NOTE: topic is logged and may contain agent identifiers — route log
// output to an access-controlled sink in multi-tenant deployments.
func (rt *RedisTransport) parseMsg(msg goredis.XMessage, topic string) (transport.Delivery, bool) {
	raw, ok := msg.Values["doc"].(string)
	if !ok || raw == "" {
		rt.opts.Logger.Printf("transport/redis: malformed entry %q on %q: missing or non-string doc field", msg.ID, topic)
		return transport.Delivery{}, false
	}
	// Guard before allocation: reject oversized entries without a heap spike.
	if len(raw) > document.MaxDocumentBytes {
		rt.opts.Logger.Printf("transport/redis: entry %q on %q exceeds max size (%d bytes), skipping", msg.ID, topic, len(raw))
		return transport.Delivery{}, false
	}
	doc, err := document.Parse([]byte(raw))
	if err != nil {
		rt.opts.Logger.Printf("transport/redis: parse error for entry %q on %q: %v", msg.ID, topic, err)
		return transport.Delivery{}, false
	}
	return transport.Delivery{Doc: doc, MsgID: msg.ID, RecvedAt: time.Now()}, true
}

// isBusyGroupErr reports whether err is a Redis BUSYGROUP error.
func isBusyGroupErr(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "BUSYGROUP")
}

// isCtxErr reports whether err is a standard context error.
func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// isNetClosedErr reports whether err signals a connection closed at the network
// layer. This cannot be checked with errors.Is because net.ErrClosed is not
// always wrapped; string matching is the accepted Go idiom here.
func isNetClosedErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}
