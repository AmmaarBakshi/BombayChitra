package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
)

// params wraps query parsing and accumulates the first validation error.
type params struct {
	q   url.Values
	r   *http.Request // set by requestParams, for checks that need the request context
	err error
}

func newParams(u *url.URL) *params { return &params{q: u.Query()} }

func requestParams(r *http.Request) *params { return &params{q: r.URL.Query(), r: r} }

func (p *params) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}

func (p *params) str(name string) string { return strings.TrimSpace(p.q.Get(name)) }

func (p *params) list(name string) []string {
	var out []string
	for _, v := range p.q[name] {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func (p *params) intRange(name string, def, lo, hi int) int {
	v := p.str(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		p.fail("%s must be an integer between %d and %d", name, lo, hi)
		return def
	}
	return n
}

func (p *params) float(name string, def, lo, hi float64) float64 {
	v := p.str(name)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < lo || f > hi {
		p.fail("%s must be a number between %g and %g", name, lo, hi)
		return def
	}
	return f
}

func (p *params) boolean(name string) bool {
	switch strings.ToLower(p.str(name)) {
	case "", "0", "false", "no":
		return false
	case "1", "true", "yes":
		return true
	}
	p.fail("%s must be true or false", name)
	return false
}

func (p *params) oneOf(name, def string, allowed ...string) string {
	v := p.str(name)
	if v == "" {
		return def
	}
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	p.fail("%s must be one of %s", name, strings.Join(allowed, ", "))
	return def
}

func (p *params) bbox() *geo.BBox {
	v := p.str("bbox")
	if v == "" {
		return nil
	}
	b, err := geo.ParseBBox(v)
	if err != nil {
		p.fail("%v", err)
		return nil
	}
	return &b
}

// point reads lat & lon; required controls whether their absence is an error.
func (p *params) point(required bool) *geo.Point {
	lat, lon := p.str("lat"), p.str("lon")
	if lat == "" && lon == "" {
		if required {
			p.fail("lat and lon are required")
		}
		return nil
	}
	pt, err := geo.ParsePoint(lat, lon)
	if err != nil {
		p.fail("%v", err)
		return nil
	}
	return &pt
}

func (p *params) limitOffset(def, max int) (int, int) {
	return p.intRange("limit", def, 1, max), p.intRange("offset", 0, 0, 100000)
}

// parseID accepts the numeric feature id (osm_id*10+type) or OSM shorthand like n123, w456, r789.
func parseID(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	types := map[byte]int64{'n': 1, 'w': 2, 'r': 3}
	if t, ok := types[raw[0]]; ok {
		id, err := strconv.ParseInt(raw[1:], 10, 64)
		if err != nil || id <= 0 {
			return 0, false
		}
		return id*10 + t, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}
