// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

// TestHindiNormalizer_CharacterByCharacter exercises every rewrite branch
// in the ported Lucene 10.4.0 algorithm. Each test case names the rule
// being exercised (see HindiNormalizer.Normalize doc-comment for the
// underlying references).
func TestHindiNormalizer_CharacterByCharacter(t *testing.T) {
	n := NewHindiNormalizer()

	cases := []struct {
		name string
		in   string
		out  string
	}{
		// NA + halant -> anusvara (dead n -> bindu)
		{"NA+virama -> anusvara", "न्", "ं"},
		// candrabindu -> anusvara
		{"candrabindu -> anusvara", "ँ", "ं"},
		// nukta deletion — combining U+093C is dropped in place
		{"bare nukta dropped", "क़", "क"},
		// Precomposed nukta variants -> base consonant
		{"precomposed qa -> ka", "क़", "क"},
		{"precomposed za -> ja", "ज़", "ज"},
		// zwj / zwnj -> delete
		{"zwj dropped", "क‍ख", "कख"},
		{"zwnj dropped", "क‌ख", "कख"},
		// virama -> delete (when not following NA handled above)
		{"trailing virama dropped", "क्", "क"},
		// chandra/short -> canonical
		{"candra E sign -> E sign", "ॅ", "े"},
		{"short E sign -> E sign", "ॆ", "े"},
		{"candra O sign -> O sign", "ॉ", "ो"},
		{"short O sign -> O sign", "ॊ", "ो"},
		{"candra E -> E", "ऍ", "ए"},
		{"short E -> E", "ऎ", "ए"},
		{"candra O -> O", "ऑ", "ओ"},
		{"short O -> O", "ऒ", "ओ"},
		{"new short A -> A", "ॲ", "अ"},
		// long -> short ind. vowels
		{"AA -> A", "आ", "अ"},
		{"II -> I", "ई", "इ"},
		{"UU -> U", "ऊ", "उ"},
		{"vocalic RR -> R", "ॠ", "ऋ"},
		{"vocalic LL -> L", "ॡ", "ऌ"},
		{"AI -> E", "ऐ", "ए"},
		{"AU -> O", "औ", "ओ"},
		// long -> short dep. vowels
		{"II sign -> I sign", "ी", "ि"},
		{"UU sign -> U sign", "ू", "ु"},
		{"vocalic RR sign -> R sign", "ॄ", "ृ"},
		{"vocalic LL sign -> L sign", "ॣ", "ॢ"},
		{"AI sign -> E sign", "ै", "े"},
		{"AU sign -> O sign", "ौ", "ो"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := n.Normalize(c.in)
			if got != c.out {
				t.Errorf("%s: input=%q want=%q got=%q", c.name, c.in, c.out, got)
			}
		})
	}
}

// TestHindiNormalizer_WordExamples covers a few multi-rune sequences that
// exercise the in-place delete behaviour from Lucene's StemmerUtil.delete.
func TestHindiNormalizer_WordExamples(t *testing.T) {
	n := NewHindiNormalizer()

	// "हिंदी" (hindii) — long II sign should collapse to short I.
	if got := n.Normalize("हिंदी"); got != "हिंदि" {
		t.Errorf("hindii normalization: got %q", got)
	}
	// "नमस्ते" — internal virama dropped.
	if got := n.Normalize("नमस्ते"); got != "नमसते" {
		t.Errorf("namaste normalization: got %q", got)
	}
	// Empty string is a no-op.
	if got := n.Normalize(""); got != "" {
		t.Errorf("empty input: got %q", got)
	}
	// ASCII passes through.
	if got := n.Normalize("abc"); got != "abc" {
		t.Errorf("ASCII passthrough: got %q", got)
	}
}
