package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/geo"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

// maxBatchQueries is the most searches one batch may hold.
const maxBatchQueries = 100

type batchRequest struct {
	Queries []string     `json:"queries"`
	Limit   int          `json:"limit"`
	Kind    []string     `json:"kind"`
	Near    *latLonPoint `json:"near"`
}

type batchResult struct {
	Query string               `json:"query"`
	Count int                  `json:"count"`
	Items []store.SearchResult `json:"items"`
}

// handleSearchBatch answers POST /api/v1/search/batch: many searches (addresses, places, stations…) in one request,
// each answered as /api/v1/search would, in the order given — batch geocoding.
func (s *Server) handleSearchBatch(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		s.badRequest(w, r, "the body must be JSON like {\"queries\": [\"CSMT\", \"Bandra station\"]}: "+err.Error())
		return
	}
	var problems []string
	if n := len(req.Queries); n == 0 || n > maxBatchQueries {
		problems = append(problems, fmt.Sprintf("give 1 to %d queries", maxBatchQueries))
	}
	if req.Limit == 0 {
		req.Limit = 1
	}
	if req.Limit < 1 || req.Limit > 10 {
		problems = append(problems, "limit must be between 1 and 10")
	}
	for _, k := range req.Kind {
		if !contains(store.SearchKinds, k) {
			problems = append(problems, "kind must be any of "+strings.Join(store.SearchKinds, ", "))
			break
		}
	}
	var near *geo.Point
	if req.Near != nil {
		if math.Abs(req.Near.Lat) > 90 || math.Abs(req.Near.Lon) > 180 {
			problems = append(problems, "near must have a valid lat and lon")
		}
		near = &geo.Point{Lon: req.Near.Lon, Lat: req.Near.Lat}
	}
	if len(problems) > 0 {
		s.badRequest(w, r, strings.Join(problems, "; "))
		return
	}

	results := make([]batchResult, len(req.Queries))
	for i, q := range req.Queries {
		q = strings.TrimSpace(q)
		items := []store.SearchResult{}
		if q != "" {
			found, err := s.store.Search(r.Context(), q, near, req.Kind, req.Limit)
			if err != nil {
				s.handleErr(w, r, err)
				return
			}
			items = found
		}
		results[i] = batchResult{Query: q, Count: len(items), Items: items}
	}
	s.writeJSON(w, r, http.StatusOK, map[string]any{"count": len(results), "results": results})
}
