package misc

import (
	"reflect"
	"testing"
)

func TestGetHighFreqTerms_BasicTruncation(t *testing.T) {
	stats := []*TermStats{
		NewTermStats("f", "a", 10, 100),
		NewTermStats("f", "b", 9, 90),
		NewTermStats("f", "c", 8, 80),
		NewTermStats("f", "d", 7, 70),
		NewTermStats("f", "e", 6, 60),
	}

	got := GetHighFreqTerms(stats, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0].Term != "a" || got[0].DocFreq != 10 {
		t.Fatalf("expected first term a (df=10), got %s (df=%d)", got[0].Term, got[0].DocFreq)
	}
	if got[1].Term != "b" || got[1].DocFreq != 9 {
		t.Fatalf("expected second term b (df=9), got %s (df=%d)", got[1].Term, got[1].DocFreq)
	}
	if got[2].Term != "c" || got[2].DocFreq != 8 {
		t.Fatalf("expected third term c (df=8), got %s (df=%d)", got[2].Term, got[2].DocFreq)
	}
}

func TestGetHighFreqTerms_TieBreakByFieldThenTerm(t *testing.T) {
	// Equal docFreq: should sort by field ascending, then term ascending.
	stats := []*TermStats{
		NewTermStats("z", "z", 5, 100),
		NewTermStats("a", "z", 5, 100),
		NewTermStats("a", "a", 5, 100),
		NewTermStats("m", "m", 5, 100),
	}

	got := GetHighFreqTerms(stats, 10)
	want := []*TermStats{
		NewTermStats("a", "a", 5, 100),
		NewTermStats("a", "z", 5, 100),
		NewTermStats("m", "m", 5, 100),
		NewTermStats("z", "z", 5, 100),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tie-break mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestGetHighFreqTerms_DifferentFieldsSameTerm(t *testing.T) {
	// Same term text, different fields, equal docFreq: field decides order.
	stats := []*TermStats{
		NewTermStats("beta", "x", 5, 50),
		NewTermStats("alpha", "x", 5, 50),
		NewTermStats("gamma", "x", 5, 50),
	}

	got := GetHighFreqTerms(stats, 10)
	if got[0].Field != "alpha" {
		t.Fatalf("expected alpha first, got %s", got[0].Field)
	}
	if got[1].Field != "beta" {
		t.Fatalf("expected beta second, got %s", got[1].Field)
	}
	if got[2].Field != "gamma" {
		t.Fatalf("expected gamma third, got %s", got[2].Field)
	}
}

func TestGetHighFreqTerms_NegativeNumTermsDefaults(t *testing.T) {
	// numTerms < 1 should default to 10.
	stats := make([]*TermStats, 5)
	for i := 0; i < 5; i++ {
		stats[i] = NewTermStats("f", string(rune('a'+i)), 5-i, int64(50-i*10))
	}
	got := GetHighFreqTerms(stats, 0)
	if len(got) != 5 {
		t.Fatalf("expected all 5 results (default 10, only 5 available), got %d", len(got))
	}
}

func TestGetHighFreqTerms_EmptySlice(t *testing.T) {
	got := GetHighFreqTerms([]*TermStats{}, 5)
	if len(got) != 0 {
		t.Fatalf("expected empty result for empty input, got %d", len(got))
	}
}

func TestGetHighFreqTermsByTotalTermFreq_BasicTruncation(t *testing.T) {
	stats := []*TermStats{
		NewTermStats("f", "a", 10, 100),
		NewTermStats("f", "b", 9, 90),
		NewTermStats("f", "c", 8, 80),
		NewTermStats("f", "d", 7, 70),
	}

	got := GetHighFreqTermsByTotalTermFreq(stats, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Term != "a" || got[0].TotalTermFreq != 100 {
		t.Fatalf("expected first term a (ttf=100), got %s (ttf=%d)", got[0].Term, got[0].TotalTermFreq)
	}
	if got[1].Term != "b" || got[1].TotalTermFreq != 90 {
		t.Fatalf("expected second term b (ttf=90), got %s (ttf=%d)", got[1].Term, got[1].TotalTermFreq)
	}
}

func TestGetHighFreqTermsByTotalTermFreq_TieBreakByFieldThenTerm(t *testing.T) {
	// Equal totalTermFreq: should sort by field ascending, then term ascending.
	stats := []*TermStats{
		NewTermStats("z", "z", 1, 50),
		NewTermStats("a", "z", 2, 50),
		NewTermStats("a", "a", 3, 50),
		NewTermStats("m", "m", 4, 50),
	}

	got := GetHighFreqTermsByTotalTermFreq(stats, 10)
	want := []*TermStats{
		NewTermStats("a", "a", 3, 50),
		NewTermStats("a", "z", 2, 50),
		NewTermStats("m", "m", 4, 50),
		NewTermStats("z", "z", 1, 50),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tie-break mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestGetHighFreqTermsByTotalTermFreq_DocFreqIgnoredOnTie(t *testing.T) {
	// totalTermFreq is equal, docFreq differs: tie-break must NOT use docFreq.
	stats := []*TermStats{
		NewTermStats("f", "b", 100, 50),
		NewTermStats("f", "a", 1, 50),
	}

	got := GetHighFreqTermsByTotalTermFreq(stats, 10)
	if got[0].Term != "a" {
		t.Fatalf("expected term a first (ascending lexicographic tie-break), got %s", got[0].Term)
	}
	if got[1].Term != "b" {
		t.Fatalf("expected term b second, got %s", got[1].Term)
	}
}
