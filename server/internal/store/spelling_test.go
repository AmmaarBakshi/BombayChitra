package store

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpellingBoost(t *testing.T) {
	cases := []struct {
		name, query string
		want        float64
	}{
		{"Vile Parle", "vileparle", 4.4},
		{"Bhandup", "bandup", 4.4},
		{"Bhandup West", "bandup", 1},
		{"Ghatkopar", "gatkopar", 4.4},
		{"Matunga", "dadar", 1},
		{"Dada Rege Marg", "dadar", 1}, // joined names must not match across word boundaries
		{"C S T Kurla Junction Flyover", "cst", 1},
		{"दादर", "dadar", 1},
	}
	for _, c := range cases {
		if got := spellingBoost(c.name, c.query); got != c.want {
			t.Errorf("spellingBoost(%q, %q) = %v, want %v", c.name, c.query, got, c.want)
		}
	}
}

func TestMatchExpressionWithSpellings(t *testing.T) {
	s := &Store{spelling: true}
	got := s.matchExpression("Vile Parle")
	want := `("vile" "parle"*) OR (norm : ("vile" "parle"*)) OR (norm : "vileparle"*)`
	if got != want {
		t.Errorf("matchExpression = %s, want %s", got, want)
	}
	if got := (&Store{}).matchExpression("Vile Parle"); got != `"vile" "parle"*` {
		t.Errorf("without spellings: %s", got)
	}
	if got := s.matchExpression("दादर"); got != `"दादर"*` {
		t.Errorf("Devanagari only: %s", got)
	}
}

// TestSpellingSharedCases checks the server folds text exactly like the pipeline.
func TestSpellingSharedCases(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "search-spelling.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	cases := 0
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		in, want, _ := strings.Cut(line, "\t")
		if got := strings.Join(spellingWords(in), " "); got != want {
			t.Errorf("spellingWords(%q) = %q, want %q", in, got, want)
		}
		cases++
	}
	if cases < 15 {
		t.Errorf("only %d cases", cases)
	}
}
