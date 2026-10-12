package routing

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
)

// Service loads the graph on first use (it is ~100 MB in memory) and bounds concurrent searches.
type Service struct {
	db    *sql.DB
	once  sync.Once
	graph *Graph
	err   error
	done  atomic.Bool
	slots chan struct{}
}

// NewService creates a lazily loading routing service over the dataset database.
func NewService(db *sql.DB, concurrency int) *Service {
	return &Service{db: db, slots: make(chan struct{}, max(1, concurrency))}
}

// Graph returns the loaded graph, loading it on the first call.
func (s *Service) Graph(ctx context.Context) (*Graph, error) {
	s.once.Do(func() {
		s.graph, s.err = Load(context.WithoutCancel(ctx), s.db)
		s.done.Store(true)
	})
	return s.graph, s.err
}

// Peek returns the graph without loading it: ok is false until the first route request has loaded it (or failed).
func (s *Service) Peek() (g *Graph, ok bool, err error) {
	if !s.done.Load() {
		return nil, false, nil
	}
	return s.graph, true, s.err
}

// Route computes a route, waiting for a free search slot.
func (s *Service) Route(ctx context.Context, fromLon, fromLat, toLon, toLat float64, opts Options) (*Result, error) {
	g, err := s.Graph(ctx)
	if err != nil {
		return nil, err
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return g.Route(fromLon, fromLat, toLon, toLat, opts)
}
