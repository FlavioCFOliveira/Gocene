// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets_test

// On-disk default-resolver acceptance test for rmp #4704 covering the three
// taxonomy-ordinal accumulators (TaxonomyFacetsAccumulator,
// ConcurrentFacetsAccumulator, RandomSamplingFacetsAccumulator).
//
// Lucene's taxonomy faceting persists each document's facet ordinals as a
// SortedNumericDocValues stream on the taxonomy index field
// (FastTaxonomyFacetCounts.countOneSegment). The accumulators' default,
// codec-driven path reads exactly that stream. Mapping ordinals back to dim
// labels additionally requires a persisted DirectoryTaxonomyReader, which is
// out of scope here (no taxonomy pipeline), so this test verifies the
// ordinal-count primitive directly via GetCount(ord) against a real on-disk
// SortedNumericDocValues field built through IndexWriter + the Lucene104 codec.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default; without it the SortedNumericDocValues are not persisted.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

type ordTestDoc struct {
	fields []interface{}
}

func (d *ordTestDoc) GetFields() []interface{} { return d.fields }

// openOrdinalIndex indexes one SortedNumericDocValues field per document with
// the supplied ordinal slices, commits, and returns an open reader plus the
// single leaf's MatchingDocs (all docs match).
func openOrdinalIndex(t *testing.T, field string, perDoc [][]int64) (*index.DirectoryReader, []*facets.MatchingDocs) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, ords := range perDoc {
		f, err := document.NewSortedNumericDocValuesField(field, ords)
		if err != nil {
			t.Fatalf("NewSortedNumericDocValuesField: %v", err)
		}
		if err := writer.AddDocument(&ordTestDoc{fields: []interface{}{f}}); err != nil {
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

	leaves, err := reader.Leaves()
	if err != nil {
		reader.Close()
		t.Fatalf("Leaves: %v", err)
	}
	md := make([]*facets.MatchingDocs, 0, len(leaves))
	for _, leaf := range leaves {
		md = append(md, facets.NewMatchingDocs(leaf, nil, leaf.Reader().MaxDoc()))
	}
	return reader, md
}

// dataset: ordinals counted across 4 docs ->
//
//	ord 1 appears in doc0, doc1, doc2  -> 3
//	ord 2 appears in doc0              -> 1
//	ord 3 appears in doc2, doc3        -> 2
var ordPerDoc = [][]int64{
	{1, 2},
	{1},
	{1, 3},
	{3},
}

func TestTaxonomyOrdinalDefaultPath_TaxonomyAccumulator(t *testing.T) {
	const field = "$facets"
	reader, md := openOrdinalIndex(t, field, ordPerDoc)
	defer reader.Close()

	taxo := facets.NewTaxonomyReader()
	tw := facets.NewTaxonomyWriterWithReader(taxo)
	// Register four ordinals so counts has room (root is ord 0).
	for i := 0; i < 4; i++ {
		if _, err := tw.AddCategory("dim/c" + string(rune('0'+i))); err != nil {
			t.Fatalf("AddCategory: %v", err)
		}
	}
	if err := tw.Commit(); err != nil {
		t.Fatalf("taxonomy commit: %v", err)
	}

	acc, err := facets.NewTaxonomyFacetsAccumulator(taxo, facets.NewFacetsConfig())
	if err != nil {
		t.Fatalf("NewTaxonomyFacetsAccumulator: %v", err)
	}
	if got := acc.GetIndexFieldName(); got != field {
		t.Fatalf("default index field = %q, want %q", got, field)
	}
	// No resolver installed: default codec-driven path must read the field.
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}

	assertOrdCounts(t, acc.GetCount)
}

func TestTaxonomyOrdinalDefaultPath_ConcurrentAccumulator(t *testing.T) {
	const field = "$facets"
	reader, md := openOrdinalIndex(t, field, ordPerDoc)
	defer reader.Close()

	acc, err := facets.NewConcurrentFacetsAccumulator(facets.NewFacetsConfig())
	if err != nil {
		t.Fatalf("NewConcurrentFacetsAccumulator: %v", err)
	}
	if got := acc.GetIndexFieldName(); got != field {
		t.Fatalf("default index field = %q, want %q", got, field)
	}
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}

	assertOrdCounts(t, acc.GetCount)
}

func TestTaxonomyOrdinalDefaultPath_RandomSamplingAccumulator(t *testing.T) {
	const field = "$facets"
	reader, md := openOrdinalIndex(t, field, ordPerDoc)
	defer reader.Close()

	// sampleRate 1.0 + minSampleSize 1 forces every (single) segment to be
	// sampled, so the sampled counts equal the exact counts.
	acc, err := facets.NewRandomSamplingFacetsAccumulator(facets.NewFacetsConfig(), 1.0)
	if err != nil {
		t.Fatalf("NewRandomSamplingFacetsAccumulator: %v", err)
	}
	acc.SetMinSampleSize(1)
	if got := acc.GetIndexFieldName(); got != field {
		t.Fatalf("default index field = %q, want %q", got, field)
	}
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}

	assertOrdCounts(t, acc.GetCount)
}

// assertOrdCounts checks the per-ordinal tallies produced from ordPerDoc.
func assertOrdCounts(t *testing.T, count func(int) int64) {
	t.Helper()
	want := map[int]int64{1: 3, 2: 1, 3: 2}
	for ord, w := range want {
		if got := count(ord); got != w {
			t.Errorf("count(ord %d) = %d, want %d", ord, got, w)
		}
	}
}
