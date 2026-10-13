package httpapi

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/pmtiles"
)

const tileCacheControl = "public, max-age=86400, stale-while-revalidate=604800"

func (s *Server) handleTileJSON(w http.ResponseWriter, r *http.Request) {
	h := s.tiles.Header()
	var meta map[string]any
	_ = json.Unmarshal(s.tiles.Metadata(), &meta)
	base := s.baseURL(r)
	tj := map[string]any{
		"tilejson": "3.0.0",
		"scheme":   "xyz",
		"tiles":    []string{base + "/tiles/{z}/{x}/{y}.mvt"},
		"minzoom":  h.MinZoom,
		"maxzoom":  h.MaxZoom,
		"bounds":   []float64{h.MinLon, h.MinLat, h.MaxLon, h.MaxLat},
		"center":   []float64{h.CenterLon, h.CenterLat, float64(h.CenterZoom)},
	}
	for _, k := range []string{"name", "description", "attribution", "version", "vector_layers"} {
		if v, ok := meta[k]; ok {
			tj[k] = v
		}
	}
	s.writeJSON(w, r, http.StatusOK, tj)
}

func (s *Server) handleTile(w http.ResponseWriter, r *http.Request) {
	z, err1 := strconv.ParseUint(r.PathValue("z"), 10, 8)
	x, err2 := strconv.ParseUint(r.PathValue("x"), 10, 32)
	yRaw := strings.TrimSuffix(strings.TrimSuffix(r.PathValue("y"), ".mvt"), ".pbf")
	y, err3 := strconv.ParseUint(yRaw, 10, 32)
	if err1 != nil || err2 != nil || err3 != nil {
		s.badRequest(w, r, "tile path must be /tiles/{z}/{x}/{y}.mvt")
		return
	}
	data, ok, err := s.tiles.Tile(uint8(z), uint32(x), uint32(y))
	if err != nil {
		if strings.Contains(err.Error(), "out of range") {
			s.badRequest(w, r, err.Error())
			return
		}
		s.handleErr(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", tileCacheControl)
	if !ok {
		// Empty tiles inside the pyramid are normal (open sea); 204 keeps browser consoles clean.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	etag := `"` + strconv.FormatUint(z, 10) + "-" + strconv.FormatUint(x, 10) + "-" + strconv.FormatUint(y, 10) + "-" +
		strconv.FormatInt(int64(len(data)), 36) + "-" + s.datasetTag() + `"`
	h.Set("ETag", etag)
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/vnd.mapbox-vector-tile")
	h.Add("Vary", "Accept-Encoding")
	if s.tiles.Header().TileCompression == pmtiles.CompressionGzip {
		if acceptsEncoding(r, "gzip") {
			h.Set("Content-Encoding", "gzip")
		} else if data, err = pmtiles.Gunzip(data); err != nil {
			s.handleErr(w, r, err)
			return
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method != http.MethodHead {
		w.Write(data)
	}
}

func (s *Server) datasetTag() string {
	v := s.store.Version()
	v = strings.NewReplacer(":", "", "-", "", "T", "", "Z", "").Replace(v)
	return v
}

// handleStyle serves the light/dark MapLibre styles produced by the web build, with {{BASE}} replaced by this
// server's public origin so the style works from any client or SDK.
func (s *Server) handleStyle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.web == nil || !strings.HasSuffix(name, ".json") || strings.ContainsAny(name, `/\`) {
		s.notFound(w, r)
		return
	}
	// ?lang=mr|hi picks the variant labelled in Marathi or Hindi (light.json → light-mr.json).
	switch lang := r.URL.Query().Get("lang"); lang {
	case "", "en":
	case "mr", "hi":
		if !strings.Contains(name, "-") {
			name = strings.TrimSuffix(name, ".json") + "-" + lang + ".json"
		}
	default:
		s.badRequest(w, r, "lang must be en, mr or hi")
		return
	}
	base := s.baseURL(r)
	key := base + "|" + name
	if cached, ok := s.styles.Load(key); ok {
		s.writeBytes(w, r, http.StatusOK, "application/json; charset=utf-8", cached.([]byte))
		return
	}
	raw, err := fs.ReadFile(s.web, "styles/"+name)
	if err != nil {
		s.notFound(w, r)
		return
	}
	rendered := bytes.ReplaceAll(raw, []byte("{{BASE}}"), []byte(base))
	s.styles.Store(key, rendered)
	s.writeBytes(w, r, http.StatusOK, "application/json; charset=utf-8", rendered)
}

// handleDownload streams the raw archive or database with HTTP range support, so clients can also read the PMTiles
// file directly (e.g. with the pmtiles JS library) or mirror the SQLite database.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	var path, contentType string
	switch r.PathValue("file") {
	case "bombaychitra.pmtiles":
		path, contentType = s.cfg.Tiles, "application/vnd.pmtiles"
	case "bombaychitra.db":
		path, contentType = s.cfg.Database, "application/vnd.sqlite3"
	default:
		s.notFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		s.notFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		s.handleErr(w, r, err)
		return
	}
	// Large downloads must not be cut off by the server-wide write timeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "public, max-age=3600")
	h.Set("ETag", `"`+s.datasetTag()+"-"+strconv.FormatInt(st.Size(), 36)+`"`)
	if r.Header.Get("Range") == "" {
		h.Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	}
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
}
