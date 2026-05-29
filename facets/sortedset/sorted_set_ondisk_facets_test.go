// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset_test

// End-to-end facets acceptance test for rmp #4704: the SortedSet facets
// accumulator must count facet ordinals by reading the field's
// SortedSetDocValues straight from each matching segment's LeafReader, with no
// bespoke test resolver installed. This exercises the default, codec-driven
// path wired in #4704 on top of the on-disk DocValues pipeline (#4771).
//
// The dataset is indexed through the real IndexWriter + Lucene104 codec and
// read back through OpenDirectoryReader, so the counts are produced by the
// production read path rather than by a hand-rolled DocValues stub. This
// mirrors the single-segment branch of Lucene 10.4.0's
// org.apache.lucene.facet.sortedset.SortedSetDocValuesFacetCounts.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/sortedset"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default; without it flushDocValues is a no-op and the field has
	// no SortedSetDocValues on read.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// ssdvTestDoc is a minimal index.Document carrying the supplied fields.
type ssdvTestDoc struct {
	fields []interface{}
}

func (d *ssdvTestDoc) GetFields() []interface{} { return d.fields }

// facetField builds a SortedSetDocValuesField whose values are the
// "dim/label" encoded terms a SortedSetDocValuesFacetField would produce.
func facetField(t *testing.T, indexField string, encoded ...string) *document.SortedSetDocValuesField {
	t.Helper()
	values := make([][]byte, len(encoded))
	for i, e := range encoded {
		values[i] = []byte(e)
	}
	f, err := document.NewSortedSetDocValuesField(indexField, values)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesField: %v", err)
	}
	return f
}

// allDocsMatchingDocs builds one MatchingDocs per leaf with every document
// matching (Bits == nil), reproducing a MatchAllDocsQuery hit set without
// needing a FacetsCollector run.
func allDocsMatchingDocs(t *testing.T, reader *index.DirectoryReader) []*facets.MatchingDocs {
	t.Helper()
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	md := make([]*facets.MatchingDocs, 0, len(leaves))
	for _, leaf := range leaves {
		md = append(md, facets.NewMatchingDocs(leaf, nil, leaf.Reader().MaxDoc()))
	}
	return md
}

// TestSortedSetFacets_OnDiskEndToEnd is the rmp #4704 acceptance test: index
// SortedSetDocValues facet terms, commit, reopen, and drive the accumulator
// with real MatchingDocs (NO resolver hook). The ordinal/label counts must
// match the expected per-label tallies.
func TestSortedSetFacets_OnDiskEndToEnd(t *testing.T) {
	const indexField = "$facets"

	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// doc0: Author/Bob, Author/Lisa
	// doc1: Author/Bob
	// doc2: Author/Lisa, Author/Frank
	// doc3: Author/Susan
	docs := [][]string{
		{"Author/Bob", "Author/Lisa"},
		{"Author/Bob"},
		{"Author/Lisa", "Author/Frank"},
		{"Author/Susan"},
	}
	for _, encoded := range docs {
		d := &ssdvTestDoc{fields: []interface{}{facetField(t, indexField, encoded...)}}
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	acc, err := sortedset.NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), indexField)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesAccumulator: %v", err)
	}
	// Crucial: no resolver installed — the accumulator must read the segment's
	// SortedSetDocValues directly from the LeafReader SPI.

	md := allDocsMatchingDocs(t, reader)
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}

	t.Run("TopChildren", func(t *testing.T) {
		result, err := acc.GetTopChildren(10, "Author")
		if err != nil {
			t.Fatalf("GetTopChildren: %v", err)
		}
		if result == nil {
			t.Fatal("GetTopChildren returned nil")
		}
		// Total = 2(Bob) + 2(Lisa) + 1(Frank) + 1(Susan) = 6 facet values.
		if result.Value != 6 {
			t.Errorf("Author total = %d, want 6", result.Value)
		}
		want := map[string]int64{"Bob": 2, "Lisa": 2, "Frank": 1, "Susan": 1}
		got := map[string]int64{}
		for _, lv := range result.LabelValues {
			got[lv.Label] = lv.Value
		}
		if len(got) != len(want) {
			t.Fatalf("label count = %d (%v), want %d", len(got), got, len(want))
		}
		for label, w := range want {
			if got[label] != w {
				t.Errorf("count(%q) = %d, want %d (all: %v)", label, got[label], w, got)
			}
		}
	})

	t.Run("SpecificValue", func(t *testing.T) {
		cases := map[string]int64{"Bob": 2, "Lisa": 2, "Frank": 1, "Susan": 1}
		for label, want := range cases {
			r, err := acc.GetSpecificValue("Author", label)
			if err != nil {
				t.Fatalf("GetSpecificValue(%q): %v", label, err)
			}
			if r.Value != want {
				t.Errorf("GetSpecificValue(Author, %q) = %d, want %d", label, r.Value, want)
			}
		}
	})
}

// TestSortedSetFacets_OnDiskRespectsBits drives the accumulator with a Bits
// filter selecting only a subset of documents, proving the default path honours
// the hit set exactly like the resolver path does.
func TestSortedSetFacets_OnDiskRespectsBits(t *testing.T) {
	const indexField = "$facets"

	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	docs := [][]string{
		{"Author/Bob"},   // doc0
		{"Author/Lisa"},  // doc1
		{"Author/Bob"},   // doc2
		{"Author/Frank"}, // doc3
	}
	for _, encoded := range docs {
		d := &ssdvTestDoc{fields: []interface{}{facetField(t, indexField, encoded...)}}
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0]

	// Match only docs 0 and 2 (both Author/Bob).
	bits := newDocBits(leaf.Reader().MaxDoc(), 0, 2)
	md := []*facets.MatchingDocs{facets.NewMatchingDocs(leaf, bits, 2)}

	acc, err := sortedset.NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), indexField)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesAccumulator: %v", err)
	}
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}

	result, err := acc.GetTopChildren(10, "Author")
	if err != nil {
		t.Fatalf("GetTopChildren: %v", err)
	}
	if result.Value != 2 {
		t.Errorf("Author total = %d, want 2 (only docs 0 and 2)", result.Value)
	}
	got := map[string]int64{}
	for _, lv := range result.LabelValues {
		got[lv.Label] = lv.Value
	}
	if got["Bob"] != 2 {
		t.Errorf("count(Bob) = %d, want 2", got["Bob"])
	}
	if _, ok := got["Lisa"]; ok {
		t.Errorf("Lisa should not be counted (doc 1 excluded by Bits): %v", got)
	}
	if _, ok := got["Frank"]; ok {
		t.Errorf("Frank should not be counted (doc 3 excluded by Bits): %v", got)
	}
}

// docBits is a minimal facets.Bits over an explicit doc-id set.
type docBits struct {
	set    map[int]struct{}
	length int
}

func newDocBits(length int, docs ...int) *docBits {
	b := &docBits{set: make(map[int]struct{}, len(docs)), length: length}
	for _, d := range docs {
		b.set[d] = struct{}{}
	}
	return b
}

func (b *docBits) Get(i int) bool { _, ok := b.set[i]; return ok }
func (b *docBits) Length() int    { return b.length }
