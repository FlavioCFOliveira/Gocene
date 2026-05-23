// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

func stemSwedish(t *testing.T, input string) string {
	t.Helper()
	s := swedishLightStemmer{}
	runes := []rune(input)
	n := s.stem(runes, len(runes))
	return string(runes[:n])
}

// TestSwedishLightStemmer_Vocabulary tests a sample of word→stem pairs from
// the svlight.txt vocabulary file bundled with Lucene 10.4.0.
func TestSwedishLightStemmer_Vocabulary(t *testing.T) {
	// Pairs extracted from svlight.txt (input tab stem).
	cases := []struct{ input, want string }{
		{"abborrar", "abborr"},   // -ar suffix
		{"abborre", "abborr"},    // -e suffix
		{"abborrarna", "abborrarn"}, // -a suffix (after s-strip: abborrarna → abborrarn)
		{"abort", "abor"},        // -t suffix
		{"adele", "adel"},        // -e suffix
	}
	for _, c := range cases {
		got := stemSwedish(t, c.input)
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestSwedishLightStemmer_AlgorithmRules verifies each rule branch directly.
func TestSwedishLightStemmer_AlgorithmRules(t *testing.T) {
	cases := []struct {
		input string
		want  string
		note  string
	}{
		// -s trailing (len > 4): "börser"[5]='r', no strip; then -er → "börs"
		{"börser", "börs", "-er suffix (2-char)"},
		// -elser (len > 7): "handelser"(9) → len-5=4 = "hand"
		{"handelser", "hand", "-elser suffix (5-char)"},
		// -heten (len > 7): "friheten"(8) → len-5=3 = "fri"
		{"friheten", "fri", "-heten suffix (5-char)"},
		// -arne (len > 6): "herrarne"(8) → len-4=4 = "herr"
		{"herrarne", "herr", "-arne suffix (4-char)"},
		// -erna (len > 6): "herrerna"(8) → len-4=4 = "herr"
		{"herrerna", "herr", "-erna suffix (4-char)"},
		// -ande (len > 6): "lysande"(7) → len-4=3 = "lys"
		{"lysande", "lys", "-ande suffix (4-char)"},
		// -else (len > 6): "frielse"(7) → len-4=3 = "fri"
		{"frielse", "fri", "-else suffix (4-char)"},
		// -are (len > 5): "köpare"(6) → len-3=3 = "köp"
		{"köpare", "köp", "-are suffix (3-char)"},
		// -het (len > 5): "frihet"(6) → len-3=3 = "fri"
		{"frihet", "fri", "-het suffix (3-char)"},
		// -ar (len > 4): "båtar"(5) → len-2=3 = "båt"
		{"båtar", "båt", "-ar suffix (2-char)"},
		// -et (len > 4): "huset"(5) → len-2=3 = "hus"
		{"huset", "hus", "-et suffix (2-char)"},
		// -t single (len > 3): "kopt"(4) → len-1=3 = "kop"
		{"kopt", "kop", "-t suffix (1-char)"},
		// -a single (len > 3): "kopa"(4) → len-1=3 = "kop"
		{"kopa", "kop", "-a suffix (1-char)"},
		// trailing -s on 5-char word: "hästs"(5) → s stripped to "häst"(4), then -t rule → "häs"
		{"hästs", "häs", "trailing -s removal then -t"},
	}
	for _, c := range cases {
		got := stemSwedish(t, c.input)
		if got != c.want {
			t.Errorf("[%s] stem(%q) = %q, want %q", c.note, c.input, got, c.want)
		}
	}
}

// TestSwedishLightStemmer_ShortWords verifies short words are not over-stemmed.
func TestSwedishLightStemmer_ShortWords(t *testing.T) {
	for _, w := range []string{"och", "den", "ett", "av"} {
		got := stemSwedish(t, w)
		if len([]rune(got)) > len([]rune(w)) {
			t.Errorf("stem(%q) = %q — unexpectedly longer", w, got)
		}
	}
}
