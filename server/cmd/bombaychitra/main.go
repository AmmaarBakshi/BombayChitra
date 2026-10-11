// Command bombaychitra serves the BombayChitra map: REST API, vector tiles, styles, web app and docs, from a single
// binary. It also exports data subsets for use in other tools.
//
//	bombaychitra serve  [--addr :8080] [--data data/build]
//	bombaychitra export --kind places --category hospital --area makabo --format geojson -o hospitals.geojson
//	bombaychitra info   [--data data/build]
//	bombaychitra download [--data data/build] [--release data-latest]
//	bombaychitra version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/AmmaarBakshi/BombayChitra/server/internal/config"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/download"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/export"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/httpapi"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/pmtiles"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/store"
	"github.com/AmmaarBakshi/BombayChitra/server/internal/webui"
)

const usage = `BombayChitra — open, detailed map of the Mumbai Metropolitan Region

Usage:
  bombaychitra serve    [flags]   run the HTTP server (API, tiles, styles, web app, docs)
  bombaychitra export   [flags]   export data as GeoJSON, NDJSON or CSV
  bombaychitra info     [flags]   print dataset information
  bombaychitra download [flags]   download the published dataset (no Java needed)
  bombaychitra version            print the version

Run "bombaychitra <command> -h" for command flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "export":
		err = runExport(os.Args[2:])
	case "info":
		err = info(os.Args[2:])
	case "download":
		err = runDownload(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("bombaychitra", httpapi.Version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func setupLogging(format, level string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func serve(args []string) error {
	cfg, err := config.Parse("serve", args)
	if err != nil {
		return err
	}
	setupLogging(cfg.LogFormat, cfg.LogLevel)

	st, err := store.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("%w\n\nBuild the data first: scripts/build-data.sh (or .ps1), or download a release", err)
	}
	defer st.Close()
	tiles, err := pmtiles.Open(cfg.Tiles)
	if err != nil {
		return err
	}
	defer tiles.Close()

	var web fs.FS = webui.FS()
	if cfg.WebDir != "" {
		web = os.DirFS(cfg.WebDir)
	}
	if web == nil {
		slog.Warn("web app not embedded; build it with `npm run build` in web/ (API and tiles still work)")
	}

	api := httpapi.New(cfg, st, tiles, web)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	slog.Info("BombayChitra listening", "addr", "http://"+displayAddr(ln.Addr().String()), "version", httpapi.Version,
		"dataset", st.Version(), "places", st.Meta()["count_places"])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 2)
	go func() { errc <- srv.Serve(ln) }()
	if cfg.Metrics && cfg.MetricsAddr != "" {
		msrv := &http.Server{Addr: cfg.MetricsAddr, Handler: api.MetricsHandler(), ReadHeaderTimeout: 5 * time.Second}
		defer msrv.Close()
		go func() { errc <- msrv.ListenAndServe() }()
		slog.Info("metrics listening", "addr", "http://"+displayAddr(cfg.MetricsAddr)+"/metrics")
	}
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	data := fs.String("data", filepath.Join("data", "build"), "data directory")
	db := fs.String("db", "", "SQLite database (default <data>/bombaychitra.db)")
	kind := fs.String("kind", "places", "what to export: "+strings.Join(export.Kinds, ", "))
	format := fs.String("format", "geojson", "output format: "+strings.Join(export.Formats, ", "))
	category := fs.String("category", "", "comma-separated categories (places only)")
	group := fs.String("group", "", "comma-separated category groups (places only)")
	area := fs.String("area", "", "restrict to an area id (e.g. makabo, navi-mumbai)")
	out := fs.String("o", "-", "output file, - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !slices.Contains(export.Kinds, *kind) {
		return fmt.Errorf("kind must be one of %s", strings.Join(export.Kinds, ", "))
	}
	if !slices.Contains(export.Formats, *format) {
		return fmt.Errorf("format must be one of %s", strings.Join(export.Formats, ", "))
	}
	if *db == "" {
		*db = filepath.Join(*data, "bombaychitra.db")
	}
	st, err := store.Open(*db)
	if err != nil {
		return err
	}
	defer st.Close()
	if *area != "" && !st.AreaExists(*area) {
		return fmt.Errorf("unknown area %q", *area)
	}
	var w io.Writer = os.Stdout
	if *out != "-" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	n, err := export.Run(context.Background(), st, export.Options{
		Kind: *kind, Format: *format, Categories: split(*category), Groups: split(*group), Area: *area,
	}, w)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "exported %d %s\n", n, *kind)
	return nil
}

func info(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	data := fs.String("data", filepath.Join("data", "build"), "data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*data, "bombaychitra.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	keys := make([]string, 0, len(st.Meta()))
	for k := range st.Meta() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Printf("%-18s %s\n", k, st.Meta()[k])
	}
	if t, err := pmtiles.Open(filepath.Join(*data, "bombaychitra.pmtiles")); err == nil {
		h := t.Header()
		fmt.Printf("%-18s z%d–z%d, %d tiles, %.1f MB\n", "tiles", h.MinZoom, h.MaxZoom, h.AddressedTiles,
			float64(t.Size())/1e6)
		t.Close()
	}
	return nil
}

func runDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	data := fs.String("data", envOr("BC_DATA", filepath.Join("data", "build")), "directory to download into [BC_DATA]")
	repo := fs.String("repo", envOr("BC_DATA_REPO", download.DefaultRepo), "GitHub repository publishing the data [BC_DATA_REPO]")
	release := fs.String("release", envOr("BC_DATA_RELEASE", download.DefaultRelease), "release tag: data-latest, or a version like v0.1.0 [BC_DATA_RELEASE]")
	url := fs.String("url", "", "download from this base URL instead of a GitHub release")
	force := fs.Bool("force", false, "download even when the local files are up to date")
	if err := fs.Parse(args); err != nil {
		return err
	}
	base := *url
	if base == "" {
		base = download.ReleaseURL(*repo, *release)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return download.Run(ctx, download.Options{BaseURL: base, Dir: *data, Force: *force, Log: os.Stderr})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
