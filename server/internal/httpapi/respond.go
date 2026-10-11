package httpapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"hash/fnv"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
)

var (
	bufPool  = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	gzipPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression); return w }}
)

const minGzipSize = 1024

// apiError is the JSON error envelope: {"error":{"status":400,"code":"bad_request","message":"..."}}.
type apiError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	s.writeBody(w, r, status, "application/json; charset=utf-8", v)
}

func (s *Server) writeGeoJSON(w http.ResponseWriter, r *http.Request, v any) {
	s.writeBody(w, r, http.StatusOK, "application/geo+json; charset=utf-8", v)
}

// writeBody encodes v, adds a content-hash ETag (answering 304 when it matches) and gzips larger bodies.
func (s *Server) writeBody(w http.ResponseWriter, r *http.Request, status int, contentType string, v any) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		slog.Error("encode response", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.writeBytes(w, r, status, contentType, buf.Bytes())
}

func (s *Server) writeBytes(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Add("Vary", "Accept-Encoding")
	if status == http.StatusOK {
		sum := fnv.New64a()
		sum.Write([]byte(s.store.Version()))
		sum.Write(body)
		etag := `W/"` + strconv.FormatUint(sum.Sum64(), 36) + `"`
		h.Set("ETag", etag)
		if h.Get("Cache-Control") == "" {
			h.Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
		}
		if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	} else {
		h.Set("Cache-Control", "no-store")
	}
	if len(body) >= minGzipSize && acceptsEncoding(r, "gzip") {
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length")
		w.WriteHeader(status)
		if r.Method == http.MethodHead {
			return
		}
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		gz.Write(body)
		gz.Close()
		gzipPool.Put(gz)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	s.writeJSON(w, r, status, map[string]apiError{"error": {Status: status, Code: code, Message: message}})
}

func (s *Server) badRequest(w http.ResponseWriter, r *http.Request, message string) {
	s.fail(w, r, http.StatusBadRequest, "bad_request", message)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.fail(w, r, http.StatusNotFound, "not_found", "resource not found")
}

// handleErr maps store errors to HTTP responses.
func (s *Server) handleErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if errors.Is(r.Context().Err(), err) || r.Context().Err() != nil {
		return // client went away
	}
	slog.Error("request failed", "path", r.URL.Path, "err", err)
	s.fail(w, r, http.StatusInternalServerError, "internal", "internal server error")
}

func acceptsEncoding(r *http.Request, enc string) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(name), enc) {
			return !strings.Contains(strings.ReplaceAll(params, " ", ""), "q=0")
		}
	}
	return false
}

// ---------------------------------------------------------------- GeoJSON

type feature struct {
	Type       string          `json:"type"`
	ID         any             `json:"id,omitempty"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties any             `json:"properties"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

func pointGeometry(lon, lat float64) json.RawMessage {
	b := make([]byte, 0, 64)
	b = append(b, `{"type":"Point","coordinates":[`...)
	b = strconv.AppendFloat(b, lon, 'f', -1, 64)
	b = append(b, ',')
	b = strconv.AppendFloat(b, lat, 'f', -1, 64)
	b = append(b, "]}"...)
	return b
}

func placesToGeoJSON(places []store.Place) featureCollection {
	fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(places))}
	for _, p := range places {
		fc.Features = append(fc.Features, feature{Type: "Feature", ID: p.ID, Geometry: pointGeometry(p.Lon, p.Lat), Properties: p})
	}
	return fc
}
