package gateway

import (
	"context"
	"log"
	"sync"
	"time"
)

// pendingEntry records a task that has been fetched from a stream but not yet
// acknowledged. The MsgID is the opaque transport-layer delivery token.
type pendingEntry struct {
	topic    string
	msgID    string
	storedAt time.Time
}

// PendingMap is a thread-safe map from taskID to the transport-layer delivery
// token needed to Ack the message when the task completes or fails.
//
// SECURITY: msgID values MUST come from the transport layer (Delivery.MsgID).
// They must never be derived from document content or Document.Extra, which is
// attacker-controlled.
type PendingMap struct {
	mu      sync.RWMutex
	entries map[string]pendingEntry
}

// NewPendingMap constructs an empty PendingMap.
func NewPendingMap() *PendingMap {
	return &PendingMap{entries: make(map[string]pendingEntry)}
}

// Store records a pending delivery for taskID on the given topic.
func (pm *PendingMap) Store(taskID, topic, msgID string) {
	pm.mu.Lock()
	pm.entries[taskID] = pendingEntry{topic: topic, msgID: msgID, storedAt: time.Now()}
	pm.mu.Unlock()
}

// Load returns the topic and msgID for a pending taskID.
// Returns ok=false if taskID is not in the map.
func (pm *PendingMap) Load(taskID string) (topic, msgID string, ok bool) {
	pm.mu.RLock()
	e, ok := pm.entries[taskID]
	pm.mu.RUnlock()
	if !ok {
		return "", "", false
	}
	return e.topic, e.msgID, true
}

// Delete removes taskID from the map after it has been acknowledged.
func (pm *PendingMap) Delete(taskID string) {
	pm.mu.Lock()
	delete(pm.entries, taskID)
	pm.mu.Unlock()
}

// StartCleanup starts a background goroutine that evicts entries older than ttl.
// It stops when ctx is cancelled.
func (pm *PendingMap) StartCleanup(ctx context.Context, ttl time.Duration) {
	go func() {
		ticker := time.NewTicker(ttl)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pm.evict(ttl)
			}
		}
	}()
}

func (pm *PendingMap) evict(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	pm.mu.Lock()
	for taskID, e := range pm.entries {
		if e.storedAt.Before(cutoff) {
			log.Printf("pending: evicting stale entry taskID=%q topic=%q (age > %v)", taskID, e.topic, ttl)
			delete(pm.entries, taskID)
		}
	}
	pm.mu.Unlock()
}
