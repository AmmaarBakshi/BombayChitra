// Package download fetches a published dataset (bombaychitra.pmtiles and bombaychitra.db) from a GitHub release,
// so a server can run without Java or a pipeline build.
//
// Files are verified against the release's SHA256SUMS, downloaded next to their destination and only renamed into
// place once every file has arrived intact, so a failed or interrupted download never leaves a mismatched pair.
package download

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRepo is the GitHub repository that publishes the weekly data release.
const DefaultRepo = "AmmaarBakshi/BombayChitra"

// DefaultRelease is the tag of the rolling weekly data release.
const DefaultRelease = "data-latest"

// Files are the dataset files a server needs.
var Files = []string{"bombaychitra.pmtiles", "bombaychitra.db"}

// ChecksumFile lists "sha256  filename" lines for the release assets.
const ChecksumFile = "SHA256SUMS"

// Options configures Run.
type Options struct {
	// BaseURL is where the files are, e.g. https://github.com/OWNER/REPO/releases/download/TAG.
	BaseURL string
	// Dir receives the files.
	Dir string
	// Force downloads files even when the local copy already matches the published checksum.
	Force bool
	// Client defaults to an HTTP client with a generous timeout for 200 MB files.
	Client *http.Client
	// Log receives progress messages; nil discards them.
	Log io.Writer
}

// ReleaseURL is the download base URL of a GitHub release.
func ReleaseURL(repo, tag string) string {
	return "https://github.com/" + repo + "/releases/download/" + tag
}

// Run downloads the dataset files that are missing or out of date.
func Run(ctx context.Context, opts Options) error {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	logf := func(format string, args ...any) {
		if opts.Log != nil {
			fmt.Fprintf(opts.Log, format+"\n", args...)
		}
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return err
	}

	sums, err := fetchChecksums(ctx, opts.Client, base+"/"+ChecksumFile)
	if err != nil {
		return err
	}
	if sums == nil {
		logf("warning: %s has no %s; files cannot be verified", base, ChecksumFile)
	}

	var fetched []string
	for _, name := range Files {
		dest := filepath.Join(opts.Dir, name)
		want := sums[name]
		if sums != nil && want == "" {
			return fmt.Errorf("%s does not list %s", ChecksumFile, name)
		}
		if !opts.Force && want != "" {
			if got, err := fileSHA256(dest); err == nil && got == want {
				logf("%s is up to date", name)
				continue
			}
		}
		logf("downloading %s", name)
		n, got, err := fetch(ctx, opts.Client, base+"/"+name, dest+".part")
		if err != nil {
			removeParts(opts.Dir, append(fetched, name))
			return fmt.Errorf("%s: %w", name, err)
		}
		if want != "" && got != want {
			removeParts(opts.Dir, append(fetched, name))
			return fmt.Errorf("%s: checksum mismatch (got %s, want %s)", name, got, want)
		}
		logf("  %.1f MB, sha256 %s", float64(n)/1e6, got[:16])
		fetched = append(fetched, name)
	}
	for _, name := range fetched {
		dest := filepath.Join(opts.Dir, name)
		if err := os.Rename(dest+".part", dest); err != nil {
			return err
		}
	}
	if len(fetched) > 0 {
		logf("dataset ready in %s; restart the server to serve it", opts.Dir)
	}
	return nil
}

// fetchChecksums returns nil (no error) when the release has no checksum file.
func fetchChecksums(ctx context.Context, client *http.Client, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, res.Status)
	}
	sums := map[string]string{}
	sc := bufio.NewScanner(io.LimitReader(res.Body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		if _, err := hex.DecodeString(fields[0]); err != nil || len(fields[0]) != 64 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return sums, sc.Err()
}

func fetch(ctx context.Context, client *http.Client, url, dest string) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, "", errors.New(res.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), res.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func removeParts(dir string, names []string) {
	for _, name := range names {
		os.Remove(filepath.Join(dir, name+".part"))
	}
}
