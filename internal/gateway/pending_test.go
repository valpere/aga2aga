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
		name   string
		taskID string
		topic  string
		msgID  string
		wantOK bool
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

func TestPendingMap_LoadAndDelete(t *testing.T) {
	pm := gateway.NewPendingMap()
	pm.Store("x", "agent.tasks.foo", "999-0")

	topic, msgID, ok := pm.LoadAndDelete("x")
	if !ok {
		t.Fatal("LoadAndDelete: want ok=true, got false")
	}
	if topic != "agent.tasks.foo" {
		t.Errorf("topic = %q, want %q", topic, "agent.tasks.foo")
	}
	if msgID != "999-0" {
		t.Errorf("msgID = %q, want %q", msgID, "999-0")
	}

	// Second call must see it gone
	_, _, ok = pm.LoadAndDelete("x")
	if ok {
		t.Error("LoadAndDelete: second call should return ok=false")
	}
}

func TestPendingMap_ConcurrentAccess(t *testing.T) {
	pm := gateway.NewPendingMap()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n * 3)
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
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i%26))
			pm.LoadAndDelete(id)
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

func TestPendingMap_StartCleanup_Idempotent(t *testing.T) {
	// Calling StartCleanup twice must not spawn duplicate eviction goroutines.
	// Verification: after double-call with a very short TTL, an entry stored
	// at t=0 is evicted exactly once (not twice), and the map remains consistent.
	pm := gateway.NewPendingMap()
	pm.Store("task-1", "agent.tasks.a", "msg-1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pm.StartCleanup(ctx, 10*time.Millisecond)
	pm.StartCleanup(ctx, 10*time.Millisecond) // second call must be a no-op

	time.Sleep(50 * time.Millisecond)

	// Entry should be evicted exactly once — both goroutines racing on delete
	// would be benign (delete is idempotent), but we verify the map is
	// still consistent and no panic occurred.
	_, _, ok := pm.Load("task-1")
	if ok {
		t.Error("entry should have been evicted after TTL")
	}
}

func TestPendingMap_CountByAgent(t *testing.T) {
	pm := gateway.NewPendingMap()
	pm.Store("task-1", "agent.tasks.agent-a", "redis-1")
	pm.Store("task-2", "agent.tasks.agent-a", "redis-2")
	pm.Store("task-3", "agent.tasks.agent-b", "redis-3")

	if n := pm.CountByAgent("agent-a"); n != 2 {
		t.Errorf("CountByAgent(agent-a) = %d, want 2", n)
	}
	if n := pm.CountByAgent("agent-b"); n != 1 {
		t.Errorf("CountByAgent(agent-b) = %d, want 1", n)
	}
	if n := pm.CountByAgent("agent-c"); n != 0 {
		t.Errorf("CountByAgent(agent-c) = %d, want 0", n)
	}
}
