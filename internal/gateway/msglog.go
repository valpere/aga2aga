package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

// MessageLogEntry carries the data captured by each gateway tool handler before
// it is written to the persistent log by the MessageLogger implementation.
type MessageLogEntry struct {
	EnvelopeID string
	ThreadID   string
	FromAgent  string
	ToAgent    string
	MsgType    string
	Direction  string // "send" | "receive"
	ToolName   string
	BodySize   int
	Body       string
}

// MessageLogger records inter-agent message traffic. Implementations must be
// non-blocking: Log returns immediately and errors are logged internally.
type MessageLogger interface {
	Log(ctx context.Context, entry MessageLogEntry)
}

// --- NoopMessageLogger ---

type noopMessageLogger struct{}

// NewNoopMessageLogger returns a zero-allocation MessageLogger that discards
// every entry. Use when --message-log=false is set.
func NewNoopMessageLogger() MessageLogger { return noopMessageLogger{} }

func (noopMessageLogger) Log(_ context.Context, _ MessageLogEntry) {}

// --- EmbeddedMessageLogger ---

// EmbeddedMessageLogger writes entries to an admin.MessageLogStore via a
// buffered channel + single drain goroutine. This keeps SQLite writes on one
// goroutine (satisfying the MaxOpenConns(1) constraint) and keeps Log()
// latency at channel-send cost only. Entries are dropped with a WARN log
// when the channel is full.
type EmbeddedMessageLogger struct {
	store  admin.MessageLogStore
	orgID  string
	ch     chan MessageLogEntry
	done   chan struct{}
	closed chan struct{}
}

const defaultLogChannelCap = 256

// NewEmbeddedMessageLogger returns a logger backed by store with the default
// channel capacity. Call Close() when the gateway shuts down.
func NewEmbeddedMessageLogger(store admin.MessageLogStore, orgID string) *EmbeddedMessageLogger {
	return NewEmbeddedMessageLoggerWithCap(store, orgID, defaultLogChannelCap)
}

// NewEmbeddedMessageLoggerWithCap is like NewEmbeddedMessageLogger but lets
// callers set the channel capacity (useful in tests).
func NewEmbeddedMessageLoggerWithCap(store admin.MessageLogStore, orgID string, cap int) *EmbeddedMessageLogger {
	l := &EmbeddedMessageLogger{
		store:  store,
		orgID:  orgID,
		ch:     make(chan MessageLogEntry, cap),
		done:   make(chan struct{}),
		closed: make(chan struct{}),
	}
	go l.drain()
	return l
}

// Log enqueues the entry for async persistence. If the channel is full the
// entry is dropped and a warning is emitted.
func (l *EmbeddedMessageLogger) Log(_ context.Context, entry MessageLogEntry) {
	select {
	case l.ch <- entry:
	default:
		log.Printf("msglog: channel full — dropping entry from=%s to=%s tool=%s",
			entry.FromAgent, entry.ToAgent, entry.ToolName)
	}
}

// Close signals the drain goroutine to flush and stop. It blocks until the
// goroutine exits.
func (l *EmbeddedMessageLogger) Close() {
	close(l.done)
	<-l.closed
}

func (l *EmbeddedMessageLogger) drain() {
	defer close(l.closed)
	for {
		select {
		case entry := <-l.ch:
			l.write(entry)
		case <-l.done:
			// Drain remaining entries before exiting.
			for {
				select {
				case entry := <-l.ch:
					l.write(entry)
				default:
					return
				}
			}
		}
	}
}

func (l *EmbeddedMessageLogger) write(entry MessageLogEntry) {
	m := &admin.MessageLog{
		ID:         uuid.NewString(),
		OrgID:      l.orgID,
		EnvelopeID: entry.EnvelopeID,
		ThreadID:   entry.ThreadID,
		FromAgent:  entry.FromAgent,
		ToAgent:    entry.ToAgent,
		MsgType:    entry.MsgType,
		Direction:  entry.Direction,
		ToolName:   entry.ToolName,
		BodySize:   entry.BodySize,
		Body:       entry.Body,
		CreatedAt:  time.Now().UTC(),
	}
	if err := l.store.AppendMessageLog(context.Background(), m); err != nil {
		log.Printf("msglog: AppendMessageLog: %v", err)
	}
}

// --- HTTPMessageLogger ---

// HTTPMessageLogger posts log entries to POST /api/v1/message-log on the admin
// server. Each call fires a goroutine (fire-and-forget); errors are logged.
type HTTPMessageLogger struct {
	endpoint string
	token    string
	client   *http.Client
}

// NewHTTPMessageLogger returns a logger that posts to baseURL+"/api/v1/message-log"
// using the given Bearer token.
func NewHTTPMessageLogger(baseURL, token string) *HTTPMessageLogger {
	return &HTTPMessageLogger{
		endpoint: baseURL + "/api/v1/message-log",
		token:    token,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (h *HTTPMessageLogger) Log(_ context.Context, entry MessageLogEntry) {
	go func() {
		body, err := json.Marshal(entry)
		if err != nil {
			log.Printf("msglog: marshal: %v", err)
			return
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.endpoint, bytes.NewReader(body))
		if err != nil {
			log.Printf("msglog: build request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+h.token)
		resp, err := h.client.Do(req)
		if err != nil {
			log.Printf("msglog: POST %s: %v", h.endpoint, err)
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			log.Printf("msglog: POST %s returned %s", h.endpoint, resp.Status)
		}
	}()
}

