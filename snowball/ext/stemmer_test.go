// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ext

import (
	"github.com/FlavioCFOliveira/Gocene/snowball"
	"testing"
)

// stem is a small helper that runs one word through a stemmer and returns the
// result, isolating the SetCurrent/Stem/GetCurrent dance.
func stem(s snowball.Stemmer, word string) string {
	s.SetCurrent(word)
	s.Stem()
	return s.GetCurrent()
}

// TestPorterStemmer drives the Porter (English) algorithm through the canonical
// examples published in Martin Porter's original paper and the Snowball
// reference vocabulary. A divergence here means the ported Among tables or the
// SnowballProgram engine no longer reproduce the reference algorithm.
func TestPorterStemmer(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		// Step 1a: plural / -es / -ies handling.
		"caresses": "caress",
		"ponies":   "poni",
		"ties":     "ti",
		"caress":   "caress",
		"cats":     "cat",
		// Step 1b: -eed / -ed / -ing.
		"feed":      "feed",
		"agreed":    "agre",
		"plastered": "plaster",
		"motoring":  "motor",
		"sing":      "sing",
		// Step 1c: terminal y -> i.
		"happy": "happi",
		// Steps 2-4: derivational suffixes.
		"relational":  "relat",
		"conditional": "condit",
		"rational":    "ration",
		"valuable":    "valuabl",
	}
	st := NewPorterStemmer()
	for in, want := range cases {
		if got := stem(st, in); got != want {
			t.Errorf("Porter(%q) = %q, want %q", in, got, want)
		}
	}

	// The stemmer is reusable: the same instance produces correct results
	// across successive words (no leftover state).
	if got := stem(st, "cats"); got != "cat" {
		t.Errorf("Porter(cats) after reuse = %q, want cat", got)
	}
}

// TestEnglishStemmer drives the English (Porter2) algorithm through a set of
// words whose stems are well established for that algorithm.
func TestEnglishStemmer(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"running":      "run",
		"jumps":        "jump",
		"easily":       "easili",
		"fairly":       "fair",
		"national":     "nation",
		"organization": "organ",
		"happily":      "happili",
	}
	st := NewEnglishStemmer()
	for in, want := range cases {
		if got := stem(st, in); got != want {
			t.Errorf("English(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEnglishStemmerIdempotent verifies that re-stemming an already-stemmed
// word yields the same result (a fixed point), a property every Snowball
// stemmer must hold.
func TestEnglishStemmerIdempotent(t *testing.T) {
	t.Parallel()
	st := NewEnglishStemmer()
	for _, word := range []string{"running", "national", "easily", "organization"} {
		once := stem(st, word)
		twice := stem(st, once)
		if once != twice {
			t.Errorf("English not idempotent for %q: %q -> %q", word, once, twice)
		}
	}
}

// TestRussianStemmer exercises a second, non-Latin algorithm to confirm the
// engine handles multi-byte runes correctly. The stems are the standard
// Russian Snowball outputs.
func TestRussianStemmer(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"вагона":   "вагон",  // genitive of "wagon" -> stem
		"вагоне":   "вагон",  // prepositional of "wagon" -> same stem
		"красивый": "красив", // "beautiful" -> stem
	}
	st := NewRussianStemmer()
	for in, want := range cases {
		if got := stem(st, in); got != want {
			t.Errorf("Russian(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFrenchStemmer exercises the French algorithm, including an irregular
// plural that the algorithm folds onto its singular stem.
func TestFrenchStemmer(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"chevaux":    "cheval",    // irregular plural -> singular stem
		"continuons": "continuon", // verb form
		"continuel":  "continuel", // already minimal under the algorithm
	}
	st := NewFrenchStemmer()
	for in, want := range cases {
		if got := stem(st, in); got != want {
			t.Errorf("French(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAllStemmers_SanityAndIdempotent iterates every Snowball stemmer in the
// ext package and asserts two universal properties:
//   1) A non-empty input never produces an empty output (sanity).
//   2) Re-stemming the output yields the same string (idempotence).
//
// Sprint 14 T86: 30 language stemmers are present; accuracy fixtures from Java
// Lucene 10.4.0 are not available for all languages, so this test provides
// the baseline validation that every stemmer is wired and algorithmically
// stable.
func TestAllStemmers_SanityAndIdempotent(t *testing.T) {
	t.Parallel()

	stemmers := []struct {
		name   string
		make   func() snowball.Stemmer
		word   string // a simple word known to exist in the language
	}{
		{"Arabic", func() snowball.Stemmer { return NewArabicStemmer() }, "كتاب"},
		{"Armenian", func() snowball.Stemmer { return NewArmenianStemmer() }, "գիրք"},
		{"Basque", func() snowball.Stemmer { return NewBasqueStemmer() }, "etxe"},
		{"Catalan", func() snowball.Stemmer { return NewCatalanStemmer() }, "llibre"},
		{"Danish", func() snowball.Stemmer { return NewDanishStemmer() }, "bog"},
		{"Dutch", func() snowball.Stemmer { return NewDutchStemmer() }, "boek"},
		{"English", func() snowball.Stemmer { return NewEnglishStemmer() }, "running"},
		{"Estonian", func() snowball.Stemmer { return NewEstonianStemmer() }, "raamat"},
		{"Finnish", func() snowball.Stemmer { return NewFinnishStemmer() }, "kirja"},
		{"French", func() snowball.Stemmer { return NewFrenchStemmer() }, "chevaux"},
		{"German", func() snowball.Stemmer { return NewGermanStemmer() }, "laufen"},
		{"Greek", func() snowball.Stemmer { return NewGreekStemmer() }, "αγαπώ"},
		{"Hindi", func() snowball.Stemmer { return NewHindiStemmer() }, "किताब"},
		{"Hungarian", func() snowball.Stemmer { return NewHungarianStemmer() }, "könyv"},
		{"Indonesian", func() snowball.Stemmer { return NewIndonesianStemmer() }, "buku"},
		{"Irish", func() snowball.Stemmer { return NewIrishStemmer() }, "leabhar"},
		{"Italian", func() snowball.Stemmer { return NewItalianStemmer() }, "libro"},
		{"Lithuanian", func() snowball.Stemmer { return NewLithuanianStemmer() }, "knyga"},
		{"Nepali", func() snowball.Stemmer { return NewNepaliStemmer() }, "पुस्तक"},
		{"Norwegian", func() snowball.Stemmer { return NewNorwegianStemmer() }, "bok"},
		{"Porter", func() snowball.Stemmer { return NewPorterStemmer() }, "running"},
		{"Portuguese", func() snowball.Stemmer { return NewPortugueseStemmer() }, "livro"},
		{"Romanian", func() snowball.Stemmer { return NewRomanianStemmer() }, "carte"},
		{"Russian", func() snowball.Stemmer { return NewRussianStemmer() }, "книга"},
		{"Serbian", func() snowball.Stemmer { return NewSerbianStemmer() }, "књига"},
		{"Spanish", func() snowball.Stemmer { return NewSpanishStemmer() }, "libro"},
		{"Swedish", func() snowball.Stemmer { return NewSwedishStemmer() }, "bok"},
		{"Tamil", func() snowball.Stemmer { return NewTamilStemmer() }, "புத்தகம்"},
		{"Turkish", func() snowball.Stemmer { return NewTurkishStemmer() }, "kitap"},
		{"Yiddish", func() snowball.Stemmer { return NewYiddishStemmer() }, "בוך"},
	}

	for _, tc := range stemmers {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := tc.make()
			once := stem(s, tc.word)
			if once == "" {
				t.Fatalf("%s stemmer produced empty output for non-empty input %q", tc.name, tc.word)
			}
			twice := stem(s, once)
			if once != twice {
				t.Errorf("%s stemmer not idempotent: %q -> %q -> %q", tc.name, tc.word, once, twice)
			}
		})
	}
}
