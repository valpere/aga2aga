package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/valpere/aga2aga/pkg/admin"
)

// LimitInfo carries the effective resource limits for an agent.
// All fields use 0 to mean "unlimited".
type LimitInfo struct {
	MaxBodyBytes    int   `json:"max_body_bytes"`
	MaxSendPerMin   int   `json:"max_send_per_min"`
	MaxPendingTasks int   `json:"max_pending_tasks"`
	MaxStreamLen    int64 `json:"max_stream_len"`
}

// LimitEnforcer gates message operations against per-agent resource limits.
// All methods must be safe for concurrent use. Implementations must be
// non-blocking: GetStreamMaxLen and GetEffectiveLimits return cached data.
type LimitEnforcer interface {
	// CheckSend returns a non-nil error if the agent is not allowed to send
	// bodySize bytes (body-size cap or rate limit exceeded).
	CheckSend(ctx context.Context, agentID string, bodySize int) error
	// RecordSend increments the agent's per-minute send counter.
	RecordSend(ctx context.Context, agentID string)
	// CheckPendingTasks returns a non-nil error if currentPending > limit
	// (strictly greater than: at-limit means one more task is still allowed).
	CheckPendingTasks(ctx context.Context, agentID string, currentPending int) error
	// GetStreamMaxLen returns the XADD MAXLEN value for the agent's stream.
	// Returns 0 when unlimited.
	GetStreamMaxLen(ctx context.Context, agentID string) int64
	// GetEffectiveLimits returns the resolved limits for agentID.
	// Never returns nil — returns zero-value LimitInfo when unconfigured.
	GetEffectiveLimits(ctx context.Context, agentID string) (*LimitInfo, error)
}

// --- NoopLimitEnforcer ---

type noopLimitEnforcer struct{}

// NewNoopLimitEnforcer returns a LimitEnforcer that allows all operations.
// Used when --enforce-limits=false.
func NewNoopLimitEnforcer() LimitEnforcer { return noopLimitEnforcer{} }

func (noopLimitEnforcer) CheckSend(_ context.Context, _ string, _ int) error { return nil }
func (noopLimitEnforcer) RecordSend(_ context.Context, _ string)              {}
func (noopLimitEnforcer) CheckPendingTasks(_ context.Context, _ string, _ int) error {
	return nil
}
func (noopLimitEnforcer) GetStreamMaxLen(_ context.Context, _ string) int64 { return 0 }
func (noopLimitEnforcer) GetEffectiveLimits(_ context.Context, _ string) (*LimitInfo, error) {
	return &LimitInfo{}, nil
}

// --- EmbeddedLimitEnforcer ---

const defaultLimitCacheTTL = 30 * time.Second

// maxMapEntries caps the number of per-agent entries in the cache and rate
// maps to prevent unbounded memory growth under high agent-ID cardinality or
// adversarial inputs (CWE-400). When the ceiling is hit, new agents get a
// throwaway bucket (permissive) or are cache-miss-fetched on every call.
const maxMapEntries = 10_000

// EmbeddedLimitEnforcer enforces limits in-process using a LimitStore.
// Effective limits are cached per agent for cacheTTL (default 30s) to avoid
// a SQLite round-trip per message. The rate counter is an in-memory
// sliding-window per agent.
type EmbeddedLimitEnforcer struct {
	store    admin.LimitStore
	orgID    string
	cacheTTL time.Duration

	mu    sync.Mutex
	cache map[string]limitCacheEntry // keyed by agentID
	rate  map[string]*rateBucket     // keyed by agentID
}

type limitCacheEntry struct {
	limits  *admin.AgentLimits
	fetchAt time.Time
}

// rateBucket tracks send timestamps for one agent within the last minute.
type rateBucket struct {
	mu        sync.Mutex
	sends     []time.Time
}

// tryRecord checks whether a new send is within the rate limit and, if so,
// records it atomically. Returns true if the send is allowed.
// limit==0 means unlimited. This single-lock operation closes the TOCTOU
// window that exists when count() and record() are called separately (CWE-362).
func (rb *rateBucket) tryRecord(limit int) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	fresh := rb.sends[:0]
	for _, t := range rb.sends {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	rb.sends = fresh
	if limit > 0 && len(rb.sends) >= limit {
		return false
	}
	rb.sends = append(rb.sends, time.Now())
	return true
}


// NewEmbeddedLimitEnforcer creates an EmbeddedLimitEnforcer with the default
// cache TTL (30s).
func NewEmbeddedLimitEnforcer(store admin.LimitStore, orgID string) *EmbeddedLimitEnforcer {
	return NewEmbeddedLimitEnforcerWithCacheTTL(store, orgID, defaultLimitCacheTTL)
}

// NewEmbeddedLimitEnforcerWithCacheTTL is like NewEmbeddedLimitEnforcer but
// allows the caller to set the cache TTL (useful in tests).
func NewEmbeddedLimitEnforcerWithCacheTTL(store admin.LimitStore, orgID string, ttl time.Duration) *EmbeddedLimitEnforcer {
	return &EmbeddedLimitEnforcer{
		store:    store,
		orgID:    orgID,
		cacheTTL: ttl,
		cache:    make(map[string]limitCacheEntry),
		rate:     make(map[string]*rateBucket),
	}
}

// effectiveLimits returns the cached limits for agentID, refreshing if stale.
func (e *EmbeddedLimitEnforcer) effectiveLimits(ctx context.Context, agentID string) *admin.AgentLimits {
	e.mu.Lock()
	entry, ok := e.cache[agentID]
	if ok && time.Since(entry.fetchAt) < e.cacheTTL {
		e.mu.Unlock()
		return entry.limits
	}
	e.mu.Unlock()

	// Fetch outside the lock to avoid holding it during a DB call.
	l, _ := e.store.GetEffectiveLimits(ctx, e.orgID, agentID)

	e.mu.Lock()
	// Only cache if below the entry ceiling (CWE-400).
	if len(e.cache) < maxMapEntries {
		e.cache[agentID] = limitCacheEntry{limits: l, fetchAt: time.Now()}
	}
	e.mu.Unlock()
	return l
}

func (e *EmbeddedLimitEnforcer) bucket(agentID string) *rateBucket {
	e.mu.Lock()
	defer e.mu.Unlock()
	if b, ok := e.rate[agentID]; ok {
		return b
	}
	// Return a throwaway bucket once the ceiling is reached so the map does
	// not grow without bound under adversarial agent-ID cardinality (CWE-400).
	if len(e.rate) >= maxMapEntries {
		return &rateBucket{}
	}
	b := &rateBucket{}
	e.rate[agentID] = b
	return b
}

// CheckSend atomically checks and records the send in a single bucket operation,
// eliminating the TOCTOU window that would exist if check and record were separate
// calls (CWE-362). The body-size check is purely read-only (no state change needed).
func (e *EmbeddedLimitEnforcer) CheckSend(ctx context.Context, agentID string, bodySize int) error {
	l := e.effectiveLimits(ctx, agentID)
	if l == nil {
		return nil
	}
	if l.MaxBodyBytes > 0 && bodySize > l.MaxBodyBytes {
		return fmt.Errorf("gateway/limits: body size %d exceeds limit %d for agent %q",
			bodySize, l.MaxBodyBytes, agentID)
	}
	if l.MaxSendPerMin > 0 {
		if !e.bucket(agentID).tryRecord(l.MaxSendPerMin) {
			return fmt.Errorf("gateway/limits: send rate limit %d/min exceeded for agent %q",
				l.MaxSendPerMin, agentID)
		}
	}
	return nil
}

// RecordSend is a no-op on EmbeddedLimitEnforcer because CheckSend already
// records atomically via tryRecord. Kept in the interface for the noop and
// HTTP stub implementations.
func (e *EmbeddedLimitEnforcer) RecordSend(_ context.Context, _ string) {}

func (e *EmbeddedLimitEnforcer) CheckPendingTasks(ctx context.Context, agentID string, currentPending int) error {
	l := e.effectiveLimits(ctx, agentID)
	if l == nil {
		return nil
	}
	if l.MaxPendingTasks > 0 && currentPending > l.MaxPendingTasks {
		return fmt.Errorf("gateway/limits: pending task limit %d reached for agent %q",
			l.MaxPendingTasks, agentID)
	}
	return nil
}

func (e *EmbeddedLimitEnforcer) GetStreamMaxLen(ctx context.Context, agentID string) int64 {
	l := e.effectiveLimits(ctx, agentID)
	if l == nil {
		return 0
	}
	return l.MaxStreamLen
}

func (e *EmbeddedLimitEnforcer) GetEffectiveLimits(ctx context.Context, agentID string) (*LimitInfo, error) {
	l := e.effectiveLimits(ctx, agentID)
	if l == nil {
		return &LimitInfo{}, nil
	}
	return &LimitInfo{
		MaxBodyBytes:    l.MaxBodyBytes,
		MaxSendPerMin:   l.MaxSendPerMin,
		MaxPendingTasks: l.MaxPendingTasks,
		MaxStreamLen:    l.MaxStreamLen,
	}, nil
}

// --- HTTPLimitEnforcer ---

// HTTPLimitEnforcer is a stub for Phase 3 remote limit enforcement.
// In Phase 2, EmbeddedLimitEnforcer is the primary implementation.
// For now it delegates all checks to NoopLimitEnforcer.
type HTTPLimitEnforcer struct {
	noopLimitEnforcer
}

// NewHTTPLimitEnforcer returns an HTTPLimitEnforcer for remote mode.
// Full implementation (delegating to admin API) is deferred to Phase 3.
func NewHTTPLimitEnforcer() *HTTPLimitEnforcer {
	return &HTTPLimitEnforcer{}
}
