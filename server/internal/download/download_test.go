package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// release serves the dataset files and, unless sums is empty, a SHA256SUMS listing them.
func release(t *testing.T, files map[string]string, sums string) (*httptest.Server, *int) {
	t.Helper()
	hits := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == ChecksumFile && sums != "" {
			w.Write([]byte(sums))
			return
		}
		body, ok := files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		*hits++
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func TestDownloadVerifiesAndSkipsUpToDate(t *testing.T) {
	files := map[string]string{"bombaychitra.pmtiles": "tiles-v2", "bombaychitra.db": "database-v2"}
	sums := digest("tiles-v2") + "  bombaychitra.pmtiles\n" + digest("database-v2") + " *bombaychitra.db\n"
	srv, hits := release(t, files, sums)
	dir := t.TempDir()

	if err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v", name, got, err)
		}
	}
	if *hits != 2 {
		t.Fatalf("downloads %d, want 2", *hits)
	}
	if err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if *hits != 2 {
		t.Errorf("up-to-date files were downloaded again (%d)", *hits)
	}
	if err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: dir, Force: true}); err != nil {
		t.Fatal(err)
	}
	if *hits != 4 {
		t.Errorf("--force downloads %d, want 4", *hits)
	}
}

func TestChecksumMismatchKeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range Files {
		os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644)
	}
	files := map[string]string{"bombaychitra.pmtiles": "tiles-v2", "bombaychitra.db": "corrupted"}
	sums := digest("tiles-v2") + "  bombaychitra.pmtiles\n" + digest("database-v2") + "  bombaychitra.db\n"
	srv, _ := release(t, files, sums)
	err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want a checksum error, got %v", err)
	}
	// Neither file was replaced, and no partial downloads are left behind.
	for _, name := range Files {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "old" {
			t.Errorf("%s was replaced: %q", name, got)
		}
		if _, err := os.Stat(filepath.Join(dir, name+".part")); !os.IsNotExist(err) {
			t.Errorf("%s.part left behind", name)
		}
	}
}

func TestWithoutChecksums(t *testing.T) {
	files := map[string]string{"bombaychitra.pmtiles": "t", "bombaychitra.db": "d"}
	srv, _ := release(t, files, "")
	var log strings.Builder
	if err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: t.TempDir(), Log: &log}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "cannot be verified") {
		t.Errorf("missing warning: %s", log.String())
	}
}

func TestMissingAsset(t *testing.T) {
	srv, _ := release(t, map[string]string{"bombaychitra.pmtiles": "t"}, "")
	if err := Run(context.Background(), Options{BaseURL: srv.URL, Dir: t.TempDir()}); err == nil {
		t.Fatal("a missing database must fail the download")
	}
}

func TestReleaseURL(t *testing.T) {
	if got := ReleaseURL("owner/repo", "data-latest"); got != "https://github.com/owner/repo/releases/download/data-latest" {
		t.Errorf("ReleaseURL = %s", got)
	}
}
