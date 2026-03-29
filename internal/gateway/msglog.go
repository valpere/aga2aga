package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

// MessageLogEntry carries the data captured by each gateway tool handler before
// it is written to the persistent log by the MessageLogger implementation.
// Direction is "send" (agent → Redis) or "receive" (Redis → agent).
// ToolName is one of the six registered MCP tool names.
type MessageLogEntry struct {
	EnvelopeID string `json:"EnvelopeID"`
	ThreadID   string `json:"ThreadID"`
	FromAgent  string `json:"FromAgent"`
	ToAgent    string `json:"ToAgent"`
	MsgType    string `json:"MsgType"`
	Direction  string `json:"Direction"`
	ToolName   string `json:"ToolName"`
	BodySize   int    `json:"BodySize"`
	Body       string `json:"Body"`
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

const httpLoggerMaxConcurrent = 32

// HTTPMessageLogger posts log entries to POST /api/v1/message-log on the admin
// server. Each call enqueues a goroutine (fire-and-forget) up to a concurrency
// cap; excess entries are dropped with a WARN log (CWE-400).
type HTTPMessageLogger struct {
	endpoint string
	token    string
	client   *http.Client
	sem      chan struct{} // limits concurrent in-flight HTTP posts
}

// NewHTTPMessageLogger returns a logger that posts to baseURL+"/api/v1/message-log"
// using the given Bearer token.
// Returns an error if baseURL is not a valid http/https URL with a non-empty host (CWE-918).
func NewHTTPMessageLogger(baseURL, token string) (*HTTPMessageLogger, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("gateway/msglog: invalid admin baseURL %q: must be http or https with non-empty host (CWE-918)", baseURL)
	}
	return &HTTPMessageLogger{
		endpoint: baseURL + "/api/v1/message-log",
		token:    token,
		client:   &http.Client{Timeout: 5 * time.Second},
		sem:      make(chan struct{}, httpLoggerMaxConcurrent),
	}, nil
}

func (h *HTTPMessageLogger) Log(_ context.Context, entry MessageLogEntry) {
	select {
	case h.sem <- struct{}{}:
	default:
		log.Printf("msglog: HTTP semaphore full — dropping entry from=%s tool=%s",
			entry.FromAgent, entry.ToolName)
		return
	}
	go func() {
		defer func() { <-h.sem }()
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
		// Drain body so the connection can be reused by http.Transport.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			log.Printf("msglog: POST %s returned %s", h.endpoint, resp.Status)
		}
	}()
}

