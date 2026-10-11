package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDefaults(t *testing.T) {
	c, err := Parse("serve", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.RateLimit != 0 || c.RequireKey || len(c.APIKeys) != 0 {
		t.Errorf("unexpected defaults: %+v", c)
	}
	if c.Database != filepath.Join("data", "build", "bombaychitra.db") {
		t.Errorf("database %q", c.Database)
	}
}

func TestRateLimitAndKeys(t *testing.T) {
	c, err := Parse("serve", []string{"--rate-limit=600", "--rate-burst=40", "--api-keys=aaaaaaaaaaaaaaaa, bbbbbbbbbbbbbbbb"})
	if err != nil {
		t.Fatal(err)
	}
	if c.RateLimit != 600 || c.RateBurst != 40 {
		t.Errorf("rate %d burst %d", c.RateLimit, c.RateBurst)
	}
	if !slices.Equal(c.APIKeys, []string{"aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"}) {
		t.Errorf("keys %q", c.APIKeys)
	}

	file := filepath.Join(t.TempDir(), "keys.txt")
	if err := os.WriteFile(file, []byte("# partners\ncccccccccccccccc\r\n\ndddddddddddddddd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Parse("serve", []string{"--api-keys=@" + file, "--require-key"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(c.APIKeys, []string{"cccccccccccccccc", "dddddddddddddddd"}) || !c.RequireKey {
		t.Errorf("keys from file %q require %v", c.APIKeys, c.RequireKey)
	}
}

func TestInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"--rate-limit=-1"},
		{"--api-keys=short"},
		{"--require-key"},
		{"--api-keys=@does-not-exist.txt"},
		{"--public-url=example.com"},
	} {
		if _, err := Parse("serve", args); err == nil {
			t.Errorf("Parse(%q) should fail", args)
		}
	}
}

func TestEnvironment(t *testing.T) {
	t.Setenv("BC_RATE_LIMIT", "120")
	t.Setenv("BC_TRUST_PROXY", "true")
	c, err := Parse("serve", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.RateLimit != 120 || !c.TrustProxy {
		t.Errorf("environment ignored: %+v", c)
	}
}
