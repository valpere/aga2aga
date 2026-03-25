package redistransport

import (
	"context"
	"errors"
	"fmt"
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
func New(client *goredis.Client, opts Options) *RedisTransport {
	opts.applyDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisTransport{client: client, opts: opts, ctx: ctx, cancel: cancel}
}

// Publish serializes doc and writes it to the named Redis stream topic via XADD.
func (rt *RedisTransport) Publish(ctx context.Context, topic string, doc *document.Document) error {
	raw, err := document.Serialize(doc)
	if err != nil {
		return fmt.Errorf("transport/redis: serialize: %w", err)
	}
	return rt.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"doc": string(raw)},
	}).Err()
}

// Subscribe creates a consumer group (idempotent) and starts a background
// XREADGROUP loop that sends Delivery values on the returned channel.
// The channel is closed when ctx is cancelled or Close() is called.
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
// topic and msgID must come from a Delivery obtained via Subscribe.
func (rt *RedisTransport) Ack(ctx context.Context, topic, msgID string) error {
	return rt.client.XAck(ctx, topic, rt.opts.ConsumerGroup, msgID).Err()
}

// Close cancels the internal context, which stops all active subscribe loops
// and closes their channels. It does not close the underlying Redis client.
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
			if isCtxErr(err) || isCancelErr(err) {
				return
			}
			if errors.Is(err, goredis.Nil) {
				// Block timeout — no new messages; loop and re-check ctx.
				continue
			}
			log.Printf("transport/redis: XReadGroup on %q: %v", topic, err)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				d, ok := rt.parseMsg(msg, topic)
				if !ok {
					// Malformed entries are skipped but remain in the PEL.
					// They will accumulate until XCLAIM or manual cleanup.
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
func (rt *RedisTransport) parseMsg(msg goredis.XMessage, topic string) (transport.Delivery, bool) {
	raw, ok := msg.Values["doc"].(string)
	if !ok || raw == "" {
		log.Printf("transport/redis: malformed entry %q on %q: missing or non-string doc field", msg.ID, topic)
		return transport.Delivery{}, false
	}
	doc, err := document.Parse([]byte(raw))
	if err != nil {
		log.Printf("transport/redis: parse error for entry %q on %q: %v", msg.ID, topic, err)
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

// isCancelErr reports whether err signals a connection closed due to cancellation.
func isCancelErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "context canceled")
}
