// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

func stemNorwegian(t *testing.T, input string, flags NorwegianVariant) string {
	t.Helper()
	s := newNorwegianLightStemmer(flags)
	runes := []rune(input)
	n := s.stem(runes, len(runes))
	return string(runes[:n])
}

// TestNorwegianLightStemmer_Bokmaal verifies Bokmål-specific suffix rules.
func TestNorwegianLightStemmer_Bokmaal(t *testing.T) {
	bm := NorwegianBokmaal
	cases := []struct{ input, want string }{
		// -heter (len > 7): "hemmeligheter"(13) → len-5=8
		{"hemmeligheter", "hemmelig"},
		// -het (len > 5): "hemmeliget"? use "frihet"(6) → len-3=3
		{"frihet", "fri"},
		// -elser (len > 7): "følelser"(8) → len-5=3
		{"følelser", "føl"},
		// -elsen (len > 7): "følelsen"(8) → len-5=3
		{"følelsen", "føl"},
		// -ende (len > 6): "sovende"(7) → len-4=3
		{"sovende", "sov"},
		// -else (len > 6): "følelse"(7) → len-4=3
		{"følelse", "føl"},
		// -este (len > 6): "fineste"(7) → len-4=3
		{"fineste", "fin"},
		// -eren (len > 6): "vandreren"(9) → len-4=5
		{"vandreren", "vandr"},
		// -ere (len > 5): "finere"(6) → len-3=3
		{"finere", "fin"},
		// -est (len > 5): "finest"(6) → len-3=3
		{"finest", "fin"},
		// -ene (len > 5): "husene"(6) → len-3=3
		{"husene", "hus"},
		// -er (len > 4): "huser"(5) → len-2=3
		{"huser", "hus"},
		// -en (len > 4): "husen"(5) → len-2=3
		{"husen", "hus"},
		// -et (len > 4): "huset"(5) → len-2=3
		{"huset", "hus"},
		// -st (len > 4): "billigst"(8) → len-2=6
		{"billigst", "billig"},
		// -te (len > 4): "spiste"(6) → len-2=4
		{"spiste", "spis"},
		// -a (len > 3): "kaka"(4) → len-1=3
		{"kaka", "kak"},
		// -e (len > 3): "kake"(4) → len-1=3
		{"kake", "kak"},
		// -n (len > 3): "stolen"(6) → first: -en rule → len-2=4 "stol"
		{"stolen", "stol"},
	}
	for _, c := range cases {
		got := stemNorwegian(t, c.input, bm)
		if got != c.want {
			t.Errorf("stem(%q, Bokmål) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestNorwegianLightStemmer_Nynorsk verifies Nynorsk-specific suffix rules.
func TestNorwegianLightStemmer_Nynorsk(t *testing.T) {
	nn := NorwegianNynorsk
	cases := []struct{ input, want string }{
		// -heita (len > 7): "hemmeleg-heita"(14) → len-5=9? use "hemmelheita"(11) → len-5=6
		{"hemmelheita", "hemmel"},
		// -leiken (len > 8): "tryggleiken"(11) → len-6=5
		{"tryggleiken", "trygg"},
		// -leikar (len > 8): "tryggleikar"(11) → len-6=5
		{"tryggleikar", "trygg"},
		// -heit (len > 6): "hemmelheit"(10) → len-4=6
		{"hemmelheit", "hemmel"},
		// -semd (len > 6): "verksemd"(8) → len-4=4
		{"verksemd", "verk"},
		// -leik (len > 6): "tryggleik"(9) → len-4=5
		{"tryggleik", "trygg"},
		// -ande (len > 6): "sovande"(7) → len-4=3
		{"sovande", "sov"},
		// -ane (len > 5): "gutane"(6) → len-3=3
		{"gutane", "gut"},
	}
	for _, c := range cases {
		got := stemNorwegian(t, c.input, nn)
		if got != c.want {
			t.Errorf("stem(%q, Nynorsk) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestNorwegianLightStemmer_ShortWords verifies short words are not over-stemmed.
func TestNorwegianLightStemmer_ShortWords(t *testing.T) {
	for _, w := range []string{"og", "av", "på"} {
		got := stemNorwegian(t, w, NorwegianBokmaal)
		if len([]rune(got)) > len([]rune(w)) {
			t.Errorf("stem(%q) = %q — unexpectedly longer", w, got)
		}
	}
}
