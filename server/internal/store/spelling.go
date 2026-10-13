package store

import (
	"strings"
	"unicode"
)

// Spelling folding for search: the server folds queries exactly as the pipeline folded names into the norm column
// of the search index (pipeline/…/util/Spelling.java). Both are tested against testdata/search-spelling.tsv.

// plainLetters maps accented Latin letters seen in Indian names to their plain forms.
var plainLetters = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ä': 'a', 'ā': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'ö': 'o', 'ō': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u',
	'ñ': 'n', 'ṇ': 'n', 'ç': 'c', 'ṭ': 't', 'ḍ': 'd',
	'ṣ': 's', 'ś': 's', 'ṛ': 'r', 'ḥ': 'h', 'ṃ': 'm',
}

// matchExpression is MatchExpression, plus — when the index has folded spellings (the norm column) — the folded
// query against them and, for several words, the words joined: "chatrapati shivaji" finds "Chhatrapati Shivaji",
// "vileparle" finds "Vile Parle".
func (s *Store) matchExpression(text string) string {
	match := MatchExpression(text)
	if match == "" || !s.spelling {
		return match
	}
	words := spellingWords(text)
	if len(words) > maxSearchTokens {
		words = words[:maxSearchTokens]
	}
	if len(words) == 0 {
		return match
	}
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + w + `"`
	}
	quoted[len(quoted)-1] += "*"
	alternatives := []string{"(" + match + ")", "(norm : (" + strings.Join(quoted, " ") + "))"}
	if len(words) > 1 {
		alternatives = append(alternatives, `(norm : "`+strings.Join(words, "")+`"*)`)
	}
	return strings.Join(alternatives, " OR ")
}

// spellingBoost rewards names that equal the query once spellings are folded and spaces removed — "vileparle" for
// "Vile Parle", "bandup" for "Bhandup" — when the literal name did not match. Such hits match only the norm column,
// which bm25 weighs 2 against the name's 10; the boost lifts them above incidental matches while a literal exact
// match ("Thane" for thane, not "Thāne") still ranks first.
func spellingBoost(name, query string) float64 {
	q := strings.Join(spellingWords(query), "")
	if q != "" && strings.Join(spellingWords(name), "") == q {
		return 2.2 * 2
	}
	return 1
}

// rank is the bm25 expression ranking search rows: the name counts most, then other names, folded spellings and
// the detail line.
func (s *Store) rank() string {
	if s.spelling {
		return "bm25(search, 10.0, 4.0, 1.0, 2.0)"
	}
	return "bm25(search, 10.0, 4.0, 1.0)"
}

var aspirates = strings.NewReplacer("kh", "k", "gh", "g", "th", "t", "dh", "d", "bh", "b", "jh", "j")

// spellingWords returns the folded words of text; words in other scripts are left out.
func spellingWords(text string) []string {
	var out []string
	var word strings.Builder
	latin := true
	flush := func() {
		if word.Len() > 0 && latin {
			out = append(out, foldWord(word.String()))
		}
		word.Reset()
		latin = true
	}
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) {
			if p, ok := plainLetters[r]; ok {
				r = p
			}
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				word.WriteRune(r)
			} else {
				latin = false
			}
			continue
		}
		flush()
	}
	flush()
	return out
}

// foldWord folds one lower-case ASCII word: chh→ch, ph→f, aspiration dropped, ee→i, oo→u, w→v, z→j, doubled letters
// single.
func foldWord(w string) string {
	w = strings.ReplaceAll(w, "chh", "ch")
	w = strings.ReplaceAll(w, "ph", "f")
	w = aspirates.Replace(w)
	w = strings.ReplaceAll(w, "ee", "i")
	w = strings.ReplaceAll(w, "oo", "u")
	w = strings.NewReplacer("w", "v", "z", "j").Replace(w)
	var b strings.Builder
	for i := 0; i < len(w); i++ {
		if i == 0 || w[i] != w[i-1] {
			b.WriteByte(w[i])
		}
	}
	return b.String()
}
