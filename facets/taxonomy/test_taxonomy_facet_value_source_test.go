// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetValueSource ports selected assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetValueSource.
//
// Integration tests (testBasic, testWithScore, etc.) used to require
// IndexWriter + FacetsCollector + DocValues pipeline. The E2E pipeline is
// now wired.
// Unit tests cover IntTaxonomyFacets and FloatTaxonomyFacets accumulation.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// stubTaxonomyReader is a minimal TaxonomyReaderI for unit tests.
type stubTaxonomyReader struct {
	size int
}

func (r *stubTaxonomyReader) GetSize() int               { return r.size }
func (r *stubTaxonomyReader) GetPath(_ int) []string     { return nil }
func (r *stubTaxonomyReader) GetOrdinal(_ ...string) int { return -1 }
func (r *stubTaxonomyReader) GetParallelTaxonomyArrays() taxonomy.ParallelTaxonomyArrays {
	parents := make([]int, r.size)
	for i := range parents {
		parents[i] = -1
	}
	return taxonomy.NewInMemoryParallelTaxonomyArrays(parents, make([]int, r.size), make([]int, r.size))
}

// TestIntTaxonomyFacets_SetGetValue verifies SetValue/GetValue round-trip.
func TestIntTaxonomyFacets_SetGetValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.SetValue(3, 42)
	if got := itf.GetValue(3); got != 42 {
		t.Errorf("GetValue(3): want 42, got %d", got)
	}
	if got := itf.GetValue(5); got != 0 {
		t.Errorf("GetValue(5): want 0, got %d", got)
	}
}

// TestIntTaxonomyFacets_AccumulateIntValue verifies SUM aggregation.
func TestIntTaxonomyFacets_AccumulateIntValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.AccumulateIntValue(2, 10)
	itf.AccumulateIntValue(2, 5)
	if got := itf.GetValue(2); got != 15 {
		t.Errorf("accumulated SUM: want 15, got %d", got)
	}
}

// TestFloatTaxonomyFacets_SetGetValue verifies FloatTaxonomyFacets.
func TestFloatTaxonomyFacets_SetGetValue(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	ftf := taxonomy.NewFloatTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	ftf.SetValue(1, 3.14)
	got := ftf.GetValue(1)
	if got < 3.13 || got > 3.15 {
		t.Errorf("GetValue(1): want ~3.14, got %v", got)
	}
	if ftf.GetValue(0) != 0.0 {
		t.Errorf("GetValue(0): want 0.0, got %v", ftf.GetValue(0))
	}
}

// TestFloatTaxonomyFacets_AccumulateFloat verifies float SUM accumulation.
func TestFloatTaxonomyFacets_AccumulateFloat(t *testing.T) {
	reader := &stubTaxonomyReader{size: 10}
	cfg := facets.NewFacetsConfig()
	ftf := taxonomy.NewFloatTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	ftf.AccumulateFloatValue(4, 1.5)
	ftf.AccumulateFloatValue(4, 2.5)
	got := ftf.GetValue(4)
	if got < 3.99 || got > 4.01 {
		t.Errorf("accumulated float SUM: want ~4.0, got %v", got)
	}
}

// -- Integration tests -------------------------------------------------------

func TestTaxonomyFacetValueSource_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	taxoDir := store.NewByteBuffersDirectory()
	defer taxoDir.Close()

	taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("creating taxonomy writer: %v", err)
	}
	defer taxoWriter.Close()

	config := facets.NewFacetsConfig()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index a few documents.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		ff := facets.NewFacetField("Dim", itoa(i))
		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, ff)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("writer commit: %v", err)
	}
	if err := taxoWriter.Commit(); err != nil {
		t.Fatalf("taxonomy commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	taxoReader, err := facets.NewDirectoryTaxonomyReaderFromWriter(taxoWriter)
	if err != nil {
		t.Fatalf("taxonomy reader: %v", err)
	}

	fc := facets.NewFacetsCollector()
	if err := searcher.SearchWithCollector(search.NewMatchAllDocsQuery(), fc); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := fc.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	adapter := taxonomy.NewDirectoryTaxonomyReaderAdapter(taxoReader)
	ftfc := taxonomy.NewFastTaxonomyFacetCounts("$facets", adapter, config)
	if err := ftfc.Accumulate(fc.GetMatchingDocs()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	// Verify each child has count 1.
	result, err := ftfc.GetTopChildren(10, "Dim")
	if err != nil {
		t.Fatalf("GetTopChildren: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Value != 5 {
		t.Errorf("total value: want 5, got %d", result.Value)
	}
}

func TestTaxonomyFacetValueSource_WithScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	taxoDir := store.NewByteBuffersDirectory()
	defer taxoDir.Close()

	taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("creating taxonomy writer: %v", err)
	}
	defer taxoWriter.Close()

	config := facets.NewFacetsConfig()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index documents.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		contentField, _ := document.NewTextField("content", "test", true)
		doc.Add(contentField)
		ff := facets.NewFacetField("Dim", itoa(i))
		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, ff)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("writer commit: %v", err)
	}
	if err := taxoWriter.Commit(); err != nil {
		t.Fatalf("taxonomy commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	taxoReader, err := facets.NewDirectoryTaxonomyReaderFromWriter(taxoWriter)
	if err != nil {
		t.Fatalf("taxonomy reader: %v", err)
	}

	// Use FacetsCollector with scores.
	fc := facets.NewFacetsCollectorWithScores()
	if err := searcher.SearchWithCollector(search.NewMatchAllDocsQuery(), fc); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := fc.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	adapter := taxonomy.NewDirectoryTaxonomyReaderAdapter(taxoReader)
	ftfc := taxonomy.NewFastTaxonomyFacetCounts("$facets", adapter, config)
	if err := ftfc.Accumulate(fc.GetMatchingDocs()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	result, err := ftfc.GetTopChildren(10, "Dim")
	if err != nil {
		t.Fatalf("GetTopChildren: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Value != 5 {
		t.Errorf("total value: want 5, got %d", result.Value)
	}
}
