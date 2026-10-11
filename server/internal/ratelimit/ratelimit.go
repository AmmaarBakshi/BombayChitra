// Package ratelimit implements per-client token buckets: each client may spend up to Burst tokens at once, refilled
// at a steady rate. Requests can cost more than one token (a route search costs more than a lookup).
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// sweepEvery is how often idle, refilled buckets are dropped so memory stays bounded by the active clients.
const sweepEvery = time.Minute

// Limiter is a set of token buckets keyed by client; safe for concurrent use.
type Limiter struct {
	rate  float64 // tokens per second
	burst float64

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
	now       func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Decision is the outcome of Allow.
type Decision struct {
	Allowed bool
	// Limit is the bucket size; Remaining the whole tokens left after this request.
	Limit     int
	Remaining int
	// RetryAfter is how long until the request would be allowed; zero when allowed.
	RetryAfter time.Duration
}

// New creates a limiter allowing perMinute tokens per minute with bursts of up to burst tokens. A burst below 1
// defaults to a tenth of the per-minute rate (at least 10).
func New(perMinute, burst int) *Limiter {
	if burst < 1 {
		burst = max(10, perMinute/10)
	}
	return &Limiter{
		rate:    float64(perMinute) / 60,
		burst:   float64(burst),
		buckets: map[string]*bucket{},
		now:     time.Now,
	}
}

// Allow spends cost tokens from key's bucket when it holds enough.
func (l *Limiter) Allow(key string, cost float64) Decision {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) >= sweepEvery {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	} else if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}
	cost = math.Min(cost, l.burst) // an expensive request must still be possible with a full bucket
	d := Decision{Limit: int(l.burst)}
	if b.tokens >= cost {
		b.tokens -= cost
		d.Allowed = true
	} else if l.rate > 0 {
		d.RetryAfter = time.Duration(math.Ceil((cost-b.tokens)/l.rate*1000)) * time.Millisecond
	} else {
		d.RetryAfter = time.Hour
	}
	d.Remaining = int(math.Floor(b.tokens))
	return d
}

// Clients returns the number of tracked buckets.
func (l *Limiter) Clients() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// sweep drops buckets that have refilled completely; they behave exactly like new ones.
func (l *Limiter) sweep(now time.Time) {
	for key, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}
