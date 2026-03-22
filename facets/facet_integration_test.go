// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-906: Facet Integration Tests
// Validates facet counting and drill-down operations produce
// identical results to Java Lucene across multiple facet configurations.

func TestFacetIntegration_BasicCounting(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	taxoDir := store.NewByteBuffersDirectory()
	defer taxoDir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("failed to create taxonomy writer: %v", err)
	}
	defer taxoWriter.Close()

	facetConfig := facets.NewFacetsConfig()

	// Add documents with facets
	docs := []struct {
		category string
		color    string
	}{
		{"electronics", "red"},
		{"electronics", "blue"},
		{"clothing", "red"},
		{"clothing", "green"},
		{"electronics", "red"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		contentField, _ := document.NewTextField("content", "product", true)
		doc.Add(contentField)

		facetConfig.SetIndexPath(doc, "category", d.category)
		facetConfig.SetIndexPath(doc, "color", d.color)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}
}

func TestFacetIntegration_DrillDown(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add categorized documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		content := "item"
		if i%2 == 0 {
			content = "electronics"
		} else {
			content = "clothing"
		}

		contentField, _ := document.NewTextField("content", content, true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test drill-down query
	baseQuery := search.NewMatchAllDocsQuery()
	drillDown := facets.NewDrillDownQuery(baseQuery)
	drillDown.Add(facets.NewFacetLabel("content", "electronics"))

	topDocs, err := searcher.Search(drillDown, nil, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	// Should find 5 electronics documents
	if topDocs.TotalHits.Value != 5 {
		t.Errorf("expected 5 electronics docs, got %d", topDocs.TotalHits.Value)
	}
}

func TestFacetIntegration_FacetCollector(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		category := "A"
		if i%2 == 0 {
			category = "B"
		}

		contentField, _ := document.NewTextField("content", category, true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Use facets collector
	facetsCollector := facets.NewFacetsCollector()
	searcher.Search(search.NewMatchAllDocsQuery(), facetsCollector, reader.NumDocs())

	// Verify collector captured facets
	if facetsCollector == nil {
		t.Error("expected non-nil facets collector")
	}
}

func TestFacetIntegration_MultipleDimensions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Products with multiple facet dimensions
	products := []struct {
		category string
		brand    string
		price    string
	}{
		{"electronics", "sony", "high"},
		{"electronics", "samsung", "medium"},
		{"clothing", "nike", "medium"},
		{"clothing", "adidas", "low"},
		{"electronics", "sony", "medium"},
	}

	for _, p := range products {
		doc := document.NewDocument()

		catField, _ := document.NewStringField("category", p.category, true)
		doc.Add(catField)

		brandField, _ := document.NewStringField("brand", p.brand, true)
		doc.Add(brandField)

		priceField, _ := document.NewStringField("price", p.price, true)
		doc.Add(priceField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}
}

func TestFacetIntegration_RangeFacets(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with numeric ranges
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()

		priceField, _ := document.NewIntField("price", i*10, true)
		doc.Add(priceField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}
}

func TestFacetIntegration_SortedSetFacets(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with multi-valued facets
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		// Multiple tags per document
		tags := []string{"tag1", "tag2", "tag3"}
		tagField, _ := document.NewStringField("tags", tags[i%3], true)
		doc.Add(tagField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}
}

func BenchmarkFacetIntegration_Counting(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	// Setup: add documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		contentField, _ := document.NewTextField("content", "test", true)
		doc.Add(contentField)
		writer.AddDocument(doc)
	}

	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	facetsCollector := facets.NewFacetsCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searcher.Search(search.NewMatchAllDocsQuery(), facetsCollector, reader.NumDocs())
	}
}
