package ratelimit

import (
	"sync"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
func newTestLimiter(perMinute, burst int) (*Limiter, *clock) {
	c := &clock{t: time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)}
	l := New(perMinute, burst)
	l.now = c.now
	return l, c
}

func TestBurstThenRefill(t *testing.T) {
	l, c := newTestLimiter(60, 5) // one token per second
	for i := 0; i < 5; i++ {
		if d := l.Allow("a", 1); !d.Allowed || d.Remaining != 4-i {
			t.Fatalf("request %d: %+v", i, d)
		}
	}
	d := l.Allow("a", 1)
	if d.Allowed || d.RetryAfter != time.Second {
		t.Fatalf("over burst: %+v", d)
	}
	c.advance(time.Second)
	if d := l.Allow("a", 1); !d.Allowed {
		t.Fatalf("after refill: %+v", d)
	}
	if d := l.Allow("b", 1); !d.Allowed || d.Remaining != 4 {
		t.Fatalf("other client shares the bucket: %+v", d)
	}
}

func TestCost(t *testing.T) {
	l, c := newTestLimiter(60, 10)
	if d := l.Allow("a", 8); !d.Allowed || d.Remaining != 2 {
		t.Fatalf("first: %+v", d)
	}
	d := l.Allow("a", 8)
	if d.Allowed || d.RetryAfter != 6*time.Second {
		t.Fatalf("second: %+v", d)
	}
	c.advance(6 * time.Second)
	if d := l.Allow("a", 8); !d.Allowed {
		t.Fatalf("after 6 s: %+v", d)
	}
	// A cost above the burst is capped, so it is never impossible.
	c.advance(time.Minute)
	if d := l.Allow("a", 50); !d.Allowed || d.Remaining != 0 {
		t.Fatalf("capped cost: %+v", d)
	}
}

func TestDefaultBurstAndSweep(t *testing.T) {
	l, c := newTestLimiter(600, 0)
	if d := l.Allow("a", 1); d.Limit != 60 {
		t.Fatalf("default burst %d, want 60", d.Limit)
	}
	l.Allow("b", 1)
	if n := l.Clients(); n != 2 {
		t.Fatalf("clients %d", n)
	}
	c.advance(2 * time.Minute)
	l.Allow("c", 1) // triggers the sweep; a and b have refilled
	if n := l.Clients(); n != 1 {
		t.Fatalf("clients after sweep %d, want 1", n)
	}
}

func TestConcurrent(t *testing.T) {
	l := New(60, 100)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("same", 1).Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed < 100 || allowed > 102 { // a token or two may refill while the goroutines run
		t.Fatalf("allowed %d of 200 with a burst of 100", allowed)
	}
}
