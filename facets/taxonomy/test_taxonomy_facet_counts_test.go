// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetCounts ports selected assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetCounts.
//
// Integration tests (testBasic, testMultiValuedHierarchy, testRandom, etc.)
// require IndexWriter + FacetsCollector + DirectoryTaxonomyReader pipeline
// and used to be deferred. The E2E pipeline is now wired.
//
// Unit tests cover TaxonomyFacets rollup logic and count increments using
// InMemoryParallelTaxonomyArrays.

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

// stubReaderFor creates a stubTaxonomyReader with the given size.
// (stubTaxonomyReader is defined in test_taxonomy_facet_value_source_test.go)
func stubReaderFor(size int) *stubTaxonomyReader {
	return &stubTaxonomyReader{size: size}
}

// TestTaxonomyFacets_ParallelArrays verifies that InMemoryParallelTaxonomyArrays
// correctly stores and returns the three parallel arrays used during rollup.
func TestTaxonomyFacets_ParallelArrays(t *testing.T) {
	parents := []int{-1, 0, 1, 1, 1}
	children := []int{1, 2, -1, -1, -1}
	siblings := []int{-1, -1, 3, 4, -1}
	arrays := taxonomy.NewInMemoryParallelTaxonomyArrays(parents, children, siblings)

	if len(arrays.Parents()) != 5 {
		t.Errorf("Parents len: want 5, got %d", len(arrays.Parents()))
	}
	if arrays.Parents()[0] != -1 {
		t.Errorf("root parent: want -1, got %d", arrays.Parents()[0])
	}
	if arrays.Children()[0] != 1 {
		t.Errorf("root first-child: want 1, got %d", arrays.Children()[0])
	}
	if arrays.Siblings()[2] != 3 {
		t.Errorf("ord2 sibling: want 3, got %d", arrays.Siblings()[2])
	}
}

// TestTaxonomyFacets_IncrCount exercises the base count increment method.
func TestTaxonomyFacets_IncrCount(t *testing.T) {
	reader := stubReaderFor(10)
	cfg := facets.NewFacetsConfig()
	itf := taxonomy.NewIntTaxonomyFacets("$facets", reader, cfg, taxonomy.SUM)

	itf.AccumulateIntValue(3, 5)
	itf.AccumulateIntValue(3, 3)
	if got := itf.GetValue(3); got != 8 {
		t.Errorf("accumulated: want 8, got %d", got)
	}
}

// parallelReader implements TaxonomyReaderI with configurable arrays.
type parallelReader struct {
	size   int
	arrays taxonomy.ParallelTaxonomyArrays
}

func (r *parallelReader) GetSize() int               { return r.size }
func (r *parallelReader) GetPath(_ int) []string     { return nil }
func (r *parallelReader) GetOrdinal(_ ...string) int { return -1 }
func (r *parallelReader) GetParallelTaxonomyArrays() taxonomy.ParallelTaxonomyArrays {
	return r.arrays
}

// -- Integration tests -------------------------------------------------------

func TestTaxonomyFacetCounts_Basic(t *testing.T) {
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
	config.SetHierarchical("Publish Date", true)

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index 5 documents as in the Java test.
	docs := []struct {
		author string
		pub    []string
	}{
		{"Bob", []string{"2010", "10", "15"}},
		{"Lisa", []string{"2010", "10", "20"}},
		{"Lisa", []string{"2012", "1", "1"}},
		{"Susan", []string{"2012", "1", "7"}},
		{"Frank", []string{"1999", "5", "5"}},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		authorFF := facets.NewFacetField("Author", d.author)
		pubFF := facets.NewFacetFieldWithPath("Publish Date", d.pub[:len(d.pub)-1], d.pub[len(d.pub)-1])

		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, authorFF, pubFF)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
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

	// Verify Publish Date result.
	result, err := ftfc.GetTopChildren(10, "Publish Date")
	if err != nil {
		t.Fatalf("GetTopChildren Publish Date: %v", err)
	}
	// Verify value and child count, then check individual entries.
	if result.Value != 5 {
		t.Errorf("Publish Date value: want 5, got %d", result.Value)
	}
	if result.ChildCount != 3 {
		t.Errorf("Publish Date childCount: want 3, got %d", result.ChildCount)
	}
	pubCounts := make(map[string]int64)
	for _, lv := range result.LabelValues {
		pubCounts[lv.Label] = lv.Value
	}
	if pubCounts["2010"] != 2 || pubCounts["2012"] != 2 || pubCounts["1999"] != 1 {
		t.Errorf("Publish Date counts: got %v", pubCounts)
	}

	// Verify Author result.
	result, err = ftfc.GetTopChildren(10, "Author")
	if err != nil {
		t.Fatalf("GetTopChildren Author: %v", err)
	}
	if result.Value != 5 {
		t.Errorf("Author value: want 5, got %d", result.Value)
	}
	if result.ChildCount != 4 {
		t.Errorf("Author childCount: want 4, got %d", result.ChildCount)
	}
	authorCounts := make(map[string]int64)
	for _, lv := range result.LabelValues {
		authorCounts[lv.Label] = lv.Value
	}
	if authorCounts["Bob"] != 1 || authorCounts["Frank"] != 1 || authorCounts["Lisa"] != 2 || authorCounts["Susan"] != 1 {
		t.Errorf("Author counts: got %v", authorCounts)
	}
}

func TestTaxonomyFacetCounts_MultiValuedHierarchy(t *testing.T) {
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
	config.SetHierarchical("Publish Date", true)
	config.SetMultiValued("Author", true)

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index documents with multi-valued authors.
	docs := []struct {
		authors []string
		pub     []string
	}{
		{[]string{"Bob", "Lisa"}, []string{"2010", "10", "15"}},
		{[]string{"Lisa"}, []string{"2010", "10", "20"}},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		var fieldList []*facets.FacetField
		for _, author := range d.authors {
			fieldList = append(fieldList, facets.NewFacetField("Author", author))
		}
		pubFF := facets.NewFacetFieldWithPath("Publish Date", d.pub[:len(d.pub)-1], d.pub[len(d.pub)-1])
		fieldList = append(fieldList, pubFF)

		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, fieldList...)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
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

	// Verify Author result. For multi-valued facets, the dimension value is -1
	// (Lucene behaviour: dimension total cannot be computed by summing children
	// when a document can contribute to multiple children), so we verify the
	// individual child counts instead.
	result, err := ftfc.GetTopChildren(10, "Author")
	if err != nil {
		t.Fatalf("GetTopChildren Author: %v", err)
	}
	if result == nil {
		t.Fatal("nil result from GetTopChildren")
	}
	authorCounts := make(map[string]int64)
	for _, lv := range result.LabelValues {
		authorCounts[lv.Label] = lv.Value
	}
	if authorCounts["Bob"] != 1 || authorCounts["Lisa"] != 2 {
		t.Errorf("Author counts: want Bob=1, Lisa=2, got %v", authorCounts)
	}
	var sum int64
	for _, v := range authorCounts {
		sum += v
	}
	if sum != 3 {
		t.Errorf("Author sum: want 3, got %d", sum)
	}
}

func TestTaxonomyFacetCounts_Random(t *testing.T) {
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

	// Index several documents and verify counts are consistent.
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		ff := facets.NewFacetField("Category", "cat_"+itoa(i%3))
		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, ff)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
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

	result, err := ftfc.GetTopChildren(10, "Category")
	if err != nil {
		t.Fatalf("GetTopChildren: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	// cat_0 appears 4 times, cat_1 3 times, cat_2 3 times
	if result.Value != 10 {
		t.Errorf("total value: want 10, got %d", result.Value)
	}
}

func TestTaxonomyFacetCounts_DrillDown(t *testing.T) {
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

	docs := []struct {
		author string
		color  string
	}{
		{"Bob", "red"},
		{"Lisa", "blue"},
		{"Lisa", "red"},
		{"Susan", "green"},
	}

	for _, d := range docs {
		doc := document.NewDocument()
		authorFF := facets.NewFacetField("Author", d.author)
		colorFF := facets.NewFacetField("Color", d.color)
		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, authorFF, colorFF)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
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

	// Search with a drill-down query.
	builder := facets.NewDrillDownQueryBuilder(config, search.NewMatchAllDocsQuery())
	builder.Add("Author", "Lisa")
	q, err := builder.Build()
	if err != nil {
		t.Fatalf("build drill-down: %v", err)
	}

	fc := facets.NewFacetsCollector()
	if err := searcher.SearchWithCollector(q, fc); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := fc.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if fc.GetTotalHits() != 2 {
		t.Errorf("drill-down total hits: want 2, got %d", fc.GetTotalHits())
	}

	// Count facets over the drill-down results.
	adapter := taxonomy.NewDirectoryTaxonomyReaderAdapter(taxoReader)
	ftfc := taxonomy.NewFastTaxonomyFacetCounts("$facets", adapter, config)
	if err := ftfc.Accumulate(fc.GetMatchingDocs()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	result, err := ftfc.GetTopChildren(10, "Color")
	if err != nil {
		t.Fatalf("GetTopChildren Color: %v", err)
	}
	if result == nil {
		t.Fatal("nil Color result")
	}

	// After drill-down to Lisa: 1 blue + 1 red = 2
	if result.Value != 2 {
		t.Errorf("Color total after drill-down: want 2, got %d", result.Value)
	}
}
