// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// buildStringFieldIndex indexes one untokenized StringField value per document,
// returning a searcher over the resulting directory. Document IDs map to the
// insertion order of values (single segment).
func buildStringFieldIndex(t *testing.T, field string, values []string) *search.IndexSearcher {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for _, v := range values {
		doc := document.NewDocument()
		sf, err := document.NewStringField(field, v, true)
		if err != nil {
			t.Fatalf("Failed to create StringField(%q): %v", v, err)
		}
		doc.Add(sf)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %q: %v", v, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	return search.NewIndexSearcher(reader)
}

func collectDocs(t *testing.T, td *search.TopDocs) []int {
	t.Helper()
	docs := make([]int, 0, len(td.ScoreDocs))
	for _, sd := range td.ScoreDocs {
		docs = append(docs, sd.Doc)
	}
	sort.Ints(docs)
	return docs
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestConstantScoreQuery_SearchTermQuery verifies that a ConstantScoreQuery
// wrapping a TermQuery returns exactly the matching documents, each scored at
// the constant score. This is the regression guard for rmp #4760: before the
// fix, ConstantScoreQuery inherited a nil-Weight CreateWeight and matched
// nothing.
func TestConstantScoreQuery_SearchTermQuery(t *testing.T) {
	const field = "f"
	// docs 0,2,4 carry "foo"; docs 1,3 carry "bar".
	searcher := buildStringFieldIndex(t, field, []string{"foo", "bar", "foo", "bar", "foo"})

	tq := search.NewTermQuery(index.NewTerm(field, "foo"))

	t.Run("default_score_1", func(t *testing.T) {
		csq := search.NewConstantScoreQuery(tq)
		td, err := searcher.Search(csq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if td.TotalHits.Value != 3 {
			t.Fatalf("Expected 3 hits, got %d", td.TotalHits.Value)
		}
		want := []int{0, 2, 4}
		if got := collectDocs(t, td); !equalInts(got, want) {
			t.Fatalf("Expected docs %v, got %v", want, got)
		}
		for _, sd := range td.ScoreDocs {
			if sd.Score != 1.0 {
				t.Errorf("doc %d: expected constant score 1.0, got %f", sd.Doc, sd.Score)
			}
		}
	}
})

	t.Run("custom_score", func(t *testing.T) {
		csq := search.NewConstantScoreQueryWithScore(tq, 2.5)
		td, err := searcher.Search(csq, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if td.TotalHits.Value != 3 {
			t.Fatalf("Expected 3 hits, got %d", td.TotalHits.Value)
		}
		for _, sd := range td.ScoreDocs {
			if sd.Score != 2.5 {
				t.Errorf("doc %d: expected constant score 2.5, got %f", sd.Doc, sd.Score)
			}
		}
	}
})
}

// TestPrefixQuery_Search verifies that a PrefixQuery enumerates the field's
// terms sharing the prefix and unions their postings. Regression guard for
// rmp #4760 / #4767: before the fix PrefixQuery wrapped itself in a
// ConstantScoreQuery and matched nothing.
func TestPrefixQuery_Search(t *testing.T) {
	const field = "f"
	// docs 0,1 share prefix "foo"; doc 2 is "other"; doc 3 is "foreign"
	// (shares "fo" but not "foo").
	searcher := buildStringFieldIndex(t, field, []string{"foobar", "foobaz", "other", "foreign"})

	q := search.NewPrefixQueryWithStrings(field, "foo")
	td, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if td.TotalHits.Value != 2 {
		t.Fatalf("Expected 2 hits for prefix 'foo', got %d", td.TotalHits.Value)
	}
	want := []int{0, 1}
	if got := collectDocs(t, td); !equalInts(got, want) {
		t.Fatalf("Expected docs %v, got %v", want, got)
	}

	// Constant score (default 1.0).
	for _, sd := range td.ScoreDocs {
		if sd.Score != 1.0 {
			t.Errorf("doc %d: expected constant score 1.0, got %f", sd.Doc, sd.Score)
		}
	}

	t.Run("prefix_matches_all_fo", func(t *testing.T) {
		q := search.NewPrefixQueryWithStrings(field, "fo")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		// foobar, foobaz, foreign all start with "fo".
		want := []int{0, 1, 3}
		if got := collectDocs(t, td); !equalInts(got, want) {
			t.Fatalf("Expected docs %v, got %v", want, got)
		}
	}
})

	t.Run("no_match", func(t *testing.T) {
		q := search.NewPrefixQueryWithStrings(field, "zzz")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if td.TotalHits.Value != 0 {
			t.Fatalf("Expected 0 hits for prefix 'zzz', got %d", td.TotalHits.Value)
		}
	}
})
}

// TestWildcardQuery_Search verifies that a WildcardQuery enumerates the field's
// terms matching the pattern and unions their postings. Regression guard for
// rmp #4760 / #4767.
func TestWildcardQuery_Search(t *testing.T) {
	const field = "f"
	// foobar, foobaz, foxbar, other
	searcher := buildStringFieldIndex(t, field, []string{"foobar", "foobaz", "foxbar", "other"})

	t.Run("star_suffix", func(t *testing.T) {
		// foo* -> foobar, foobaz
		q := search.NewWildcardQueryWithStrings(field, "foo*")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		want := []int{0, 1}
		if got := collectDocs(t, td); !equalInts(got, want) {
			t.Fatalf("Expected docs %v, got %v", want, got)
		}
		for _, sd := range td.ScoreDocs {
			if sd.Score != 1.0 {
				t.Errorf("doc %d: expected constant score 1.0, got %f", sd.Doc, sd.Score)
			}
		}
	}
})

	t.Run("single_char", func(t *testing.T) {
		// fo?bar -> foobar (foo?bar would not match; fo?bar matches "foxbar"
		// and "foobar" since '?' matches exactly one char).
		q := search.NewWildcardQueryWithStrings(field, "fo?bar")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		// "foobar" (o), "foxbar" (x) both match fo?bar; "foobaz" does not.
		want := []int{0, 2}
		if got := collectDocs(t, td); !equalInts(got, want) {
			t.Fatalf("Expected docs %v, got %v", want, got)
		}
	}
})

	t.Run("star_inside", func(t *testing.T) {
		// f*bar -> foobar, foxbar
		q := search.NewWildcardQueryWithStrings(field, "f*bar")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		want := []int{0, 2}
		if got := collectDocs(t, td); !equalInts(got, want) {
			t.Fatalf("Expected docs %v, got %v", want, got)
		}
	}
})

	t.Run("no_match", func(t *testing.T) {
		q := search.NewWildcardQueryWithStrings(field, "zzz*")
		td, err := searcher.Search(q, 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		if td.TotalHits.Value != 0 {
			t.Fatalf("Expected 0 hits for 'zzz*', got %d", td.TotalHits.Value)
		}
	}
})
}