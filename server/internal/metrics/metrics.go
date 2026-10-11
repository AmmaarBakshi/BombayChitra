// Package metrics is a small, dependency-free implementation of the Prometheus text exposition format: labelled
// counters and histograms plus gauges computed when scraped. It covers what the server needs without pulling in the
// Prometheus client library.
package metrics

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Registry holds metrics in registration order.
type Registry struct {
	mu      sync.Mutex
	metrics []metric
	names   map[string]bool
}

type metric interface {
	write(w *bufio.Writer)
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry { return &Registry{names: map[string]bool{}} }

func (r *Registry) add(name string, m metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names[name] {
		panic("metrics: duplicate metric " + name)
	}
	r.names[name] = true
	r.metrics = append(r.metrics, m)
}

// WriteTo writes every metric in the Prometheus text format (version 0.0.4).
func (r *Registry) WriteTo(w io.Writer) (int64, error) {
	r.mu.Lock()
	metrics := slices.Clone(r.metrics)
	r.mu.Unlock()
	cw := &countingWriter{w: w}
	bw := bufio.NewWriter(cw)
	for _, m := range metrics {
		m.write(bw)
	}
	err := bw.Flush()
	return cw.n, err
}

// ContentType is the media type of WriteTo's output.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func header(w *bufio.Writer, name, help, kind string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, escapeHelp(help), name, kind)
}

// ---------------------------------------------------------------- counters

// CounterVec is a family of counters partitioned by label values.
type CounterVec struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	values     map[string]*series
}

type series struct {
	labels []string
	value  float64
	// histogram state
	buckets []uint64
	sum     float64
	count   uint64
}

// NewCounterVec registers a counter family.
func (r *Registry) NewCounterVec(name, help string, labels ...string) *CounterVec {
	c := &CounterVec{name: name, help: help, labels: labels, values: map[string]*series{}}
	r.add(name, c)
	return c
}

// Add increases the counter for the label values by v (v must not be negative).
func (c *CounterVec) Add(v float64, labelValues ...string) {
	if v < 0 {
		panic("metrics: counter " + c.name + " cannot decrease")
	}
	c.mu.Lock()
	lookup(c.values, c.labels, labelValues).value += v
	c.mu.Unlock()
}

// Inc adds one.
func (c *CounterVec) Inc(labelValues ...string) { c.Add(1, labelValues...) }

func (c *CounterVec) write(w *bufio.Writer) {
	header(w, c.name, c.help, "counter")
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range sorted(c.values) {
		fmt.Fprintf(w, "%s%s %s\n", c.name, labelString(c.labels, s.labels, "", ""), formatFloat(s.value))
	}
}

// ---------------------------------------------------------------- histograms

// HistogramVec is a family of histograms with fixed upper bounds.
type HistogramVec struct {
	name, help string
	labels     []string
	bounds     []float64
	mu         sync.Mutex
	values     map[string]*series
}

// DefaultBuckets suit HTTP latencies in seconds.
var DefaultBuckets = []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// NewHistogramVec registers a histogram family; bounds must be increasing.
func (r *Registry) NewHistogramVec(name, help string, bounds []float64, labels ...string) *HistogramVec {
	if !slices.IsSorted(bounds) {
		panic("metrics: histogram bounds must be sorted")
	}
	h := &HistogramVec{name: name, help: help, labels: labels, bounds: slices.Clone(bounds), values: map[string]*series{}}
	r.add(name, h)
	return h
}

// Observe records one value.
func (h *HistogramVec) Observe(v float64, labelValues ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := lookup(h.values, h.labels, labelValues)
	if s.buckets == nil {
		s.buckets = make([]uint64, len(h.bounds))
	}
	for i, b := range h.bounds {
		if v <= b {
			s.buckets[i]++
		}
	}
	s.sum += v
	s.count++
}

func (h *HistogramVec) write(w *bufio.Writer) {
	header(w, h.name, h.help, "histogram")
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range sorted(h.values) {
		for i, b := range h.bounds {
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, labelString(h.labels, s.labels, "le", formatFloat(b)), s.buckets[i])
		}
		fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, labelString(h.labels, s.labels, "le", "+Inf"), s.count)
		fmt.Fprintf(w, "%s_sum%s %s\n", h.name, labelString(h.labels, s.labels, "", ""), formatFloat(s.sum))
		fmt.Fprintf(w, "%s_count%s %d\n", h.name, labelString(h.labels, s.labels, "", ""), s.count)
	}
}

// ---------------------------------------------------------------- gauges

// GaugeFunc is a gauge whose value is computed at scrape time.
type GaugeFunc struct {
	name, help string
	labels     []string
	fn         func() []Sample
}

// Sample is one labelled gauge value.
type Sample struct {
	Labels []string
	Value  float64
}

// NewGaugeFunc registers a gauge family computed by fn when scraped.
func (r *Registry) NewGaugeFunc(name, help string, labels []string, fn func() []Sample) {
	r.add(name, &GaugeFunc{name: name, help: help, labels: labels, fn: fn})
}

func (g *GaugeFunc) write(w *bufio.Writer) {
	header(w, g.name, g.help, "gauge")
	for _, s := range g.fn() {
		fmt.Fprintf(w, "%s%s %s\n", g.name, labelString(g.labels, s.Labels, "", ""), formatFloat(s.Value))
	}
}

// ---------------------------------------------------------------- helpers

func lookup(values map[string]*series, names, labelValues []string) *series {
	if len(labelValues) != len(names) {
		panic(fmt.Sprintf("metrics: got %d label values for %d labels", len(labelValues), len(names)))
	}
	key := strings.Join(labelValues, "\xff")
	s, ok := values[key]
	if !ok {
		s = &series{labels: slices.Clone(labelValues)}
		values[key] = s
	}
	return s
}

func sorted(values map[string]*series) []*series {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]*series, len(keys))
	for i, k := range keys {
		out[i] = values[k]
	}
	return out
}

func labelString(names, values []string, extraName, extraValue string) string {
	if len(names) == 0 && extraName == "" {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
		b.WriteString(`="`)
		b.WriteString(escapeLabel(values[i]))
		b.WriteByte('"')
	}
	if extraName != "" {
		if len(names) > 0 {
			b.WriteByte(',')
		}
		b.WriteString(extraName)
		b.WriteString(`="`)
		b.WriteString(extraValue)
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

var (
	labelEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)
	helpEscaper  = strings.NewReplacer(`\`, `\\`, "\n", `\n`)
)

func escapeLabel(s string) string { return labelEscaper.Replace(s) }
func escapeHelp(s string) string  { return helpEscaper.Replace(s) }

func formatFloat(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	case math.IsNaN(v):
		return "NaN"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
