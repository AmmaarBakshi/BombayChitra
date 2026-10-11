package httpapi

import (
	"bytes"
	"hash/fnv"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// staticHandler serves the built web app: precompressed .br/.gz variants when the client accepts them, immutable
// caching for content-hashed assets, content-hash ETags for everything else, and clean URLs for /docs pages.
type staticHandler struct {
	fsys  fs.FS
	etags sync.Map // name → etag
}

func newStaticHandler(fsys fs.FS) *staticHandler { return &staticHandler{fsys: fsys} }

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	name, ok := h.resolve(name)
	if !ok {
		// Unknown extension-less paths fall back to the app shell (client-side routes); others are 404s.
		if path.Ext(r.URL.Path) == "" {
			name = "index.html"
		} else {
			http.NotFound(w, r)
			return
		}
	}

	header := w.Header()
	ctype := mime.TypeByExtension(path.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	header.Set("Content-Type", ctype)
	switch {
	case strings.HasPrefix(name, "assets/"):
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	case strings.HasPrefix(name, "fonts/"), strings.HasPrefix(name, "icons/"):
		header.Set("Cache-Control", "public, max-age=2592000")
	default:
		header.Set("Cache-Control", "no-cache")
	}

	served := name
	if compressible(ctype) {
		header.Add("Vary", "Accept-Encoding")
		for _, enc := range []struct{ name, ext string }{{"br", ".br"}, {"gzip", ".gz"}} {
			if acceptsEncoding(r, enc.name) && h.exists(name+enc.ext) {
				served = name + enc.ext
				header.Set("Content-Encoding", enc.name)
				break
			}
		}
	}
	etag, err := h.etag(served)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	header.Set("ETag", etag)
	f, err := h.fsys.Open(served)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "", time.Time{}, rs)
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// resolve maps a URL path to a file: exact file, directory index, or "name.html".
func (h *staticHandler) resolve(name string) (string, bool) {
	if name == "" || name == "." {
		return "index.html", h.exists("index.html")
	}
	if st, err := fs.Stat(h.fsys, name); err == nil {
		if !st.IsDir() {
			return name, true
		}
		idx := path.Join(name, "index.html")
		return idx, h.exists(idx)
	}
	if h.exists(name + ".html") {
		return name + ".html", true
	}
	return "", false
}

func (h *staticHandler) exists(name string) bool {
	st, err := fs.Stat(h.fsys, name)
	return err == nil && !st.IsDir()
}

// etag hashes a file's content once and caches the result.
func (h *staticHandler) etag(name string) (string, error) {
	if v, ok := h.etags.Load(name); ok {
		return v.(string), nil
	}
	f, err := h.fsys.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := fnv.New64a()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	tag := `"` + strconv.FormatUint(sum.Sum64(), 36) + `"`
	h.etags.Store(name, tag)
	return tag, nil
}

func compressible(ctype string) bool {
	return strings.HasPrefix(ctype, "text/") || strings.Contains(ctype, "javascript") ||
		strings.Contains(ctype, "json") || strings.Contains(ctype, "svg") || strings.Contains(ctype, "xml") ||
		strings.Contains(ctype, "wasm") || strings.Contains(ctype, "protobuf")
}

func init() {
	// Platform MIME tables vary (notably on Windows); pin the types the web build relies on.
	for ext, t := range map[string]string{
		".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
		".css": "text/css; charset=utf-8", ".html": "text/html; charset=utf-8", ".json": "application/json",
		".svg": "image/svg+xml", ".pbf": "application/x-protobuf", ".webmanifest": "application/manifest+json",
		".woff2": "font/woff2", ".png": "image/png", ".ico": "image/x-icon", ".yaml": "application/yaml",
		".txt": "text/plain; charset=utf-8", ".md": "text/markdown; charset=utf-8",
	} {
		_ = mime.AddExtensionType(ext, t)
	}
}
