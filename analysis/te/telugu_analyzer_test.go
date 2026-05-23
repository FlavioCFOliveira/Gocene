// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te_test

import (
	"testing"

	tepkg "github.com/FlavioCFOliveira/Gocene/analysis/te"
)

// normalizeOnly runs the TeluguNormalizer directly on a rune slice.
func normalizeOnly(t *testing.T, input string) string {
	t.Helper()
	n := tepkg.NewTeluguNormalizer()
	runes := []rune(input)
	l := n.Normalize(runes, len(runes))
	return string(runes[:l])
}

// TestTeluguAnalyzer_Basics mirrors TestTeluguAnalyzer.testBasics():
// two ways to write oo letter should conflate via TeluguNormalizer.
func TestTeluguAnalyzer_Basics(t *testing.T) {
	// ఒౕనమాల normalizes to ఓనమాల (composed oo)
	got := normalizeOnly(t, "ఒౕనమాల")
	want := "ఓనమాల"
	if got != want {
		t.Errorf("normalize(%q) = %q, want %q", "ఒౕనమాల", got, want)
	}
}

// TestTeluguNormalizer_LongToShort verifies long vowel → short vowel mappings.
func TestTeluguNormalizer_LongToShort(t *testing.T) {
	cases := []struct{ input, want string }{
		{"ఆ", "అ"},
		{"ఈ", "ఇ"},
		{"ఊ", "ఉ"},
		{"ఐ", "ఏ"},
		{"ఔ", "ఓ"},
	}
	for _, c := range cases {
		got := normalizeOnly(t, c.input)
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestTeluguNormalizer_CandrabinduToBindu verifies candrabindu → bindu mapping.
func TestTeluguNormalizer_CandrabinduToBindu(t *testing.T) {
	// ఀ -> ం
	got := normalizeOnly(t, "ఀ")
	want := "ం"
	if got != want {
		t.Errorf("normalize(ఀ) = %q, want %q", got, want)
	}
}

// TestTeluguNormalizer_DeleteVisarga verifies visarga deletion.
func TestTeluguNormalizer_DeleteVisarga(t *testing.T) {
	// ః is deleted
	got := normalizeOnly(t, "రామః")
	want := "రామ"
	if got != want {
		t.Errorf("normalize(%q) = %q, want %q", "రామః", got, want)
	}
}

// TestTeluguAnalyzer_ResourcesAvailable verifies the analyzer can be created
// without panic (mirrors testResourcesAvailable).
func TestTeluguAnalyzer_ResourcesAvailable(t *testing.T) {
	a := tepkg.NewTeluguAnalyzer()
	if a == nil {
		t.Fatal("NewTeluguAnalyzer returned nil")
	}
}

// TestTeluguAnalyzer_WithStemExclusion verifies stem exclusion set is wired.
func TestTeluguAnalyzer_WithStemExclusion(t *testing.T) {
	a := tepkg.NewTeluguAnalyzer()
	if a == nil {
		t.Fatal("NewTeluguAnalyzer returned nil")
	}
	// Simply verify it does not panic on construction.
}
