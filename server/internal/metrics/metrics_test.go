package metrics

import (
	"strings"
	"sync"
	"testing"
)

func TestExposition(t *testing.T) {
	r := NewRegistry()
	requests := r.NewCounterVec("app_requests_total", "Requests served.", "route", "code")
	latency := r.NewHistogramVec("app_latency_seconds", "Request latency.", []float64{0.1, 1}, "route")
	r.NewGaugeFunc("app_up", "Whether the app is up.", nil, func() []Sample { return []Sample{{Value: 1}} })

	requests.Inc("/b", "200")
	requests.Add(2, "/a", "200")
	requests.Inc("/a", "404")
	latency.Observe(0.05, "/a")
	latency.Observe(0.5, "/a")
	latency.Observe(3, "/a")

	var b strings.Builder
	if _, err := r.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	want := `# HELP app_requests_total Requests served.
# TYPE app_requests_total counter
app_requests_total{route="/a",code="200"} 2
app_requests_total{route="/a",code="404"} 1
app_requests_total{route="/b",code="200"} 1
# HELP app_latency_seconds Request latency.
# TYPE app_latency_seconds histogram
app_latency_seconds_bucket{route="/a",le="0.1"} 1
app_latency_seconds_bucket{route="/a",le="1"} 2
app_latency_seconds_bucket{route="/a",le="+Inf"} 3
app_latency_seconds_sum{route="/a"} 3.55
app_latency_seconds_count{route="/a"} 3
# HELP app_up Whether the app is up.
# TYPE app_up gauge
app_up 1
`
	if b.String() != want {
		t.Errorf("exposition:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestEscaping(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounterVec("x_total", "Line one\nline \\ two.", "path")
	c.Inc(`a"b\c` + "\n")
	var b strings.Builder
	r.WriteTo(&b)
	if !strings.Contains(b.String(), `# HELP x_total Line one\nline \\ two.`) {
		t.Errorf("help not escaped: %s", b.String())
	}
	if !strings.Contains(b.String(), `x_total{path="a\"b\\c\n"} 1`) {
		t.Errorf("label not escaped: %s", b.String())
	}
}

func TestMisuse(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounterVec("dup_total", "x", "a")
	mustPanic(t, "duplicate name", func() { r.NewCounterVec("dup_total", "x") })
	mustPanic(t, "wrong label count", func() { c.Inc() })
	mustPanic(t, "negative add", func() { c.Add(-1, "v") })
	mustPanic(t, "unsorted buckets", func() { r.NewHistogramVec("h", "x", []float64{2, 1}) })
}

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected a panic", name)
		}
	}()
	fn()
}

func TestConcurrentUpdates(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounterVec("n_total", "x", "k")
	h := r.NewHistogramVec("d_seconds", "x", DefaultBuckets, "k")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Inc("k")
				h.Observe(0.01, "k")
			}
		}()
	}
	wg.Wait()
	var b strings.Builder
	r.WriteTo(&b)
	if !strings.Contains(b.String(), `n_total{k="k"} 5000`) || !strings.Contains(b.String(), `d_seconds_count{k="k"} 5000`) {
		t.Errorf("lost updates:\n%s", b.String())
	}
}
