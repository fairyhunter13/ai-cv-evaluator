package redpanda

import (
	"testing"
	"time"
)

func TestAdaptivePoller_SuccessAndFailureIntervals(t *testing.T) {
	base := 2 * time.Second
	p := NewAdaptivePoller(base)

	// Initial interval should be around base when no history
	iv := p.GetNextInterval()
	if iv < p.minInterval || iv > p.maxInterval {
		t.Fatalf("initial interval out of range: %v", iv)
	}

	// After several successes, interval should decrease but not below minInterval
	for i := 0; i < 3; i++ {
		p.RecordSuccess()
	}
	iv = p.GetNextInterval()
	if iv < p.minInterval || iv > base {
		t.Fatalf("success interval out of range: %v (min=%v, base=%v)", iv, p.minInterval, base)
	}
	if !p.IsHealthy() {
		t.Fatalf("poller should be healthy after successes")
	}

	// After several failures, interval should back off up to maxInterval
	for i := 0; i < 5; i++ {
		p.RecordFailure()
	}
	iv = p.GetNextInterval()
	if iv <= base || iv > p.maxInterval {
		t.Fatalf("failure backoff interval out of range: %v (base=%v, max=%v)", iv, base, p.maxInterval)
	}

	// Hit circuit breaker threshold
	for i := 0; i < 10; i++ {
		p.RecordFailure()
	}
	iv = p.GetNextInterval()
	if iv != p.maxInterval {
		t.Fatalf("expected circuit breaker interval %v, got %v", p.maxInterval, iv)
	}
	if p.IsHealthy() {
		t.Fatalf("poller should be unhealthy after many failures")
	}
}

func TestAdaptivePoller_GetStatsAndReset(t *testing.T) {
	p := NewAdaptivePoller(1 * time.Second)
	p.RecordSuccess()
	p.RecordFailure()

	stats := p.GetStats()
	if stats["total_polls"].(int) != 2 {
		t.Fatalf("expected total_polls=2, got %v", stats["total_polls"])
	}

	p.Reset()
	stats = p.GetStats()
	if stats["success_count"].(int) != 0 || stats["failure_count"].(int) != 0 {
		t.Fatalf("expected counters reset to 0, got %+v", stats)
	}
	if !p.IsHealthy() {
		t.Fatalf("poller should be healthy after reset")
	}
}

func TestAdaptivePollingManager_GetPollerReuseAndStats(t *testing.T) {
	m := NewAdaptivePollingManager(10 * time.Minute)

	p1 := m.GetPoller("topic-1", time.Second)
	p2 := m.GetPoller("topic-1", time.Second)
	if p1 != p2 {
		t.Fatalf("expected GetPoller to reuse existing poller")
	}

	p3 := m.GetPoller("topic-2", time.Second)
	p3.RecordSuccess()

	stats := m.GetAllStats()
	if len(stats) != 2 {
		t.Fatalf("expected stats for 2 topics, got %d", len(stats))
	}
}

func TestAdaptivePollingManager_CleanupOldPollers(t *testing.T) {
	m := &AdaptivePollingManager{
		pollers: make(map[string]*AdaptivePoller),
	}

	old := NewAdaptivePoller(time.Second)
	old.lastPollTime = time.Now().Add(-2 * time.Hour)
	fresh := NewAdaptivePoller(time.Second)
	fresh.lastPollTime = time.Now()

	m.pollers["old"] = old
	m.pollers["fresh"] = fresh

	m.cleanupOldPollers()

	if _, ok := m.pollers["old"]; ok {
		t.Fatalf("expected old poller to be cleaned up")
	}
	if _, ok := m.pollers["fresh"]; !ok {
		t.Fatalf("expected fresh poller to remain")
	}
}

func TestAdaptivePollingManager_Stop(t *testing.T) {
	m := NewAdaptivePollingManager(50 * time.Millisecond)
	// Ensure Stop does not panic and closes cleanup channel
	m.Stop()
}
