// Package config resolves server settings from flags, falling back to BC_* environment variables.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds runtime settings for the HTTP server.
type Config struct {
	Addr       string   // listen address, e.g. ":8080"
	DataDir    string   // directory containing the build artefacts
	Tiles      string   // PMTiles archive path (defaults to DataDir/bombaychitra.pmtiles)
	Database   string   // SQLite path (defaults to DataDir/bombaychitra.db)
	PublicURL  string   // absolute base URL used in TileJSON/styles; derived from the request when empty
	TrustProxy bool     // honour X-Forwarded-Proto/Host when deriving the base URL
	CORSOrigin string   // Access-Control-Allow-Origin for data endpoints ("" disables CORS)
	WebDir     string   // serve the web app from this directory instead of the embedded build
	Downloads  bool     // expose the raw .pmtiles and .db files under /data/
	LogFormat  string   // "text" or "json"
	LogLevel   string   // "debug", "info", "warn", "error"
	RateLimit  int      // API requests per minute per client; 0 disables rate limiting
	RateBurst  int      // requests a client may make at once; 0 derives it from RateLimit
	APIKeys    []string // keys whose holders bypass the rate limit
	RequireKey bool     // reject API requests without a valid key
}

// Parse reads flags (with environment defaults) for the serve command.
func Parse(name string, args []string) (Config, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var c Config
	fs.StringVar(&c.Addr, "addr", env("BC_ADDR", ":8080"), "listen address [BC_ADDR]")
	fs.StringVar(&c.DataDir, "data", env("BC_DATA", filepath.Join("data", "build")), "data directory [BC_DATA]")
	fs.StringVar(&c.Tiles, "tiles", env("BC_TILES", ""), "PMTiles archive (default <data>/bombaychitra.pmtiles) [BC_TILES]")
	fs.StringVar(&c.Database, "db", env("BC_DB", ""), "SQLite database (default <data>/bombaychitra.db) [BC_DB]")
	fs.StringVar(&c.PublicURL, "public-url", env("BC_PUBLIC_URL", ""), "public base URL, e.g. https://maps.example.com [BC_PUBLIC_URL]")
	fs.BoolVar(&c.TrustProxy, "trust-proxy", envBool("BC_TRUST_PROXY", false), "trust X-Forwarded-* headers [BC_TRUST_PROXY]")
	fs.StringVar(&c.CORSOrigin, "cors", env("BC_CORS", "*"), "CORS allowed origin for data endpoints, empty to disable [BC_CORS]")
	fs.StringVar(&c.WebDir, "web", env("BC_WEB", ""), "serve the web app from a directory instead of the embedded build [BC_WEB]")
	fs.BoolVar(&c.Downloads, "downloads", envBool("BC_DOWNLOADS", true), "expose raw data files under /data/ [BC_DOWNLOADS]")
	fs.StringVar(&c.LogFormat, "log-format", env("BC_LOG_FORMAT", "text"), "log format: text or json [BC_LOG_FORMAT]")
	fs.StringVar(&c.LogLevel, "log-level", env("BC_LOG_LEVEL", "info"), "log level [BC_LOG_LEVEL]")
	fs.IntVar(&c.RateLimit, "rate-limit", envInt("BC_RATE_LIMIT", 0), "API requests per minute per client, 0 to disable [BC_RATE_LIMIT]")
	fs.IntVar(&c.RateBurst, "rate-burst", envInt("BC_RATE_BURST", 0), "API requests a client may make at once (default rate-limit/10) [BC_RATE_BURST]")
	keys := fs.String("api-keys", env("BC_API_KEYS", ""), "comma-separated API keys, or @file with one key per line [BC_API_KEYS]")
	fs.BoolVar(&c.RequireKey, "require-key", envBool("BC_REQUIRE_KEY", false), "reject API requests without a valid key [BC_REQUIRE_KEY]")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	var err error
	if c.APIKeys, err = parseKeys(*keys); err != nil {
		return c, err
	}
	if c.RequireKey && len(c.APIKeys) == 0 {
		return c, fmt.Errorf("require-key needs at least one key in api-keys")
	}
	if c.Tiles == "" {
		c.Tiles = filepath.Join(c.DataDir, "bombaychitra.pmtiles")
	}
	if c.Database == "" {
		c.Database = filepath.Join(c.DataDir, "bombaychitra.db")
	}
	if c.RateLimit < 0 || c.RateBurst < 0 {
		return c, fmt.Errorf("rate-limit and rate-burst must not be negative")
	}
	c.PublicURL = strings.TrimRight(c.PublicURL, "/")
	if c.PublicURL != "" && !strings.HasPrefix(c.PublicURL, "http://") && !strings.HasPrefix(c.PublicURL, "https://") {
		return c, fmt.Errorf("public-url must start with http:// or https://")
	}
	return c, nil
}

// parseKeys reads "k1,k2" or "@path" (one key per line, # comments allowed).
func parseKeys(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	sep := ","
	if file, ok := strings.CutPrefix(v, "@"); ok {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("api-keys: %w", err)
		}
		v, sep = string(data), "\n"
	}
	var keys []string
	for _, k := range strings.Split(v, sep) {
		k = strings.TrimSpace(k)
		if k == "" || strings.HasPrefix(k, "#") {
			continue
		}
		if len(k) < 16 {
			return nil, fmt.Errorf("api-keys: keys must be at least 16 characters")
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
