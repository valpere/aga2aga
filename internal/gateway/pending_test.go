package gateway_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/valpere/aga2aga/internal/gateway"
)

func TestPendingMap_StoreLoadDelete(t *testing.T) {
	tests := []struct {
		name    string
		taskID  string
		topic   string
		msgID   string
		wantOK  bool
	}{
		{name: "store and load", taskID: "t1", topic: "agent.tasks.foo", msgID: "123-0", wantOK: true},
		{name: "load missing", taskID: "t2", topic: "", msgID: "", wantOK: false},
	}

	pm := gateway.NewPendingMap()

	pm.Store("t1", "agent.tasks.foo", "123-0")

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			topic, msgID, ok := pm.Load(tc.taskID)
			if ok != tc.wantOK {
				t.Errorf("Load(%q) ok = %v, want %v", tc.taskID, ok, tc.wantOK)
			}
			if ok {
				if topic != tc.topic {
					t.Errorf("topic = %q, want %q", topic, tc.topic)
				}
				if msgID != tc.msgID {
					t.Errorf("msgID = %q, want %q", msgID, tc.msgID)
				}
			}
		})
	}

	pm.Delete("t1")
	_, _, ok := pm.Load("t1")
	if ok {
		t.Error("Load after Delete should return ok=false")
	}
}

func TestPendingMap_ConcurrentAccess(t *testing.T) {
	pm := gateway.NewPendingMap()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 2)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i%26))
			pm.Store(id, "topic", "msg-"+id)
		}(i)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i%26))
			pm.Load(id)
		}(i)
	}
	wg.Wait()
}

func TestPendingMap_TTLCleanup(t *testing.T) {
	pm := gateway.NewPendingMap()
	pm.Store("old", "topic", "msg-old")

	// Use a very short TTL so the entry expires quickly
	pm.StartCleanup(t.Context(), 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	_, _, ok := pm.Load("old")
	if ok {
		t.Error("entry should have been evicted after TTL")
	}
}

func TestPendingMap_StartCleanupStopsOnContextCancel(t *testing.T) {
	pm := gateway.NewPendingMap()
	ctx, cancel := context.WithCancel(context.Background())
	pm.StartCleanup(ctx, time.Hour)
	cancel() // should not block or panic
	time.Sleep(5 * time.Millisecond)
}
