// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestOrdinalData ports the test assertions from
// org.apache.lucene.facet.taxonomy.TestOrdinalData.
//
// Full integration tests (testDocValue, testSearchableField, testReindex)
// used to require IndexWriter + NumericDocValuesField + DirectoryTaxonomyReader.
// The E2E pipeline is now wired.
// Unit tests cover ReindexingEnrichedDirectoryTaxonomyWriter metadata API.

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

// mockWriter implements taxonomy.Writer for unit testing.
type mockWriter struct {
	nextOrd int
	closed  bool
}

func (m *mockWriter) AddCategory(_ string, _ []string) (int, error) {
	ord := m.nextOrd
	m.nextOrd++
	return ord, nil
}
func (m *mockWriter) Commit() error { return nil }
func (m *mockWriter) Close() error  { m.closed = true; return nil }

// TestReindexingEnrichedWriter_AddCategory verifies AddCategory delegates and
// tracks ordinals.
func TestReindexingEnrichedWriter_AddCategory(t *testing.T) {
	mock := &mockWriter{nextOrd: 1}
	w := taxonomy.NewReindexingEnrichedDirectoryTaxonomyWriter(mock)

	ord, err := w.AddCategory("Author", []string{"Bob"})
	if err != nil {
		t.Fatalf("AddCategory: %v", err)
	}
	if ord != 1 {
		t.Errorf("ord: want 1, got %d", ord)
	}

	ord, err = w.AddCategory("Author", []string{"Lisa"})
	if err != nil {
		t.Fatalf("AddCategory: %v", err)
	}
	if ord != 2 {
		t.Errorf("ord: want 2, got %d", ord)
	}
}

// TestReindexingEnrichedWriter_Metadata verifies PutMetadata / GetMetadata.
func TestReindexingEnrichedWriter_Metadata(t *testing.T) {
	mock := &mockWriter{nextOrd: 0}
	w := taxonomy.NewReindexingEnrichedDirectoryTaxonomyWriter(mock)

	// Initially metadata is nil.
	if w.GetMetadata(0) != nil {
		t.Error("expected nil metadata before AddCategory")
	}

	w.AddCategory("A", []string{"1"}) //nolint:errcheck
	w.PutMetadata(0, []byte{0x01, 0x02})

	got := w.GetMetadata(0)
	if len(got) != 2 || got[0] != 0x01 || got[1] != 0x02 {
		t.Errorf("GetMetadata(0): want [1 2], got %v", got)
	}
}

// TestReindexingEnrichedWriter_Close verifies Close delegates.
func TestReindexingEnrichedWriter_Close(t *testing.T) {
	mock := &mockWriter{}
	w := taxonomy.NewReindexingEnrichedDirectoryTaxonomyWriter(mock)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mock.closed {
		t.Error("expected mock writer to be closed")
	}
}

// -- Integration tests -------------------------------------------------------

func TestOrdinalData_DocValue(t *testing.T) {
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

	// Index documents with facet categories and verify ordinal-based access.
	for i := 0; i < 3; i++ {
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

	// Verify taxonomy contains the expected categories.
	taxoReader, err := facets.NewDirectoryTaxonomyReaderFromWriter(taxoWriter)
	if err != nil {
		t.Fatalf("taxonomy reader: %v", err)
	}

	if sz := taxoReader.GetSize(); sz < 4 {
		t.Errorf("taxonomy size: want >= 4 (root+dim+3 children), got %d", sz)
	}

	// Verify ordinals are accessible.
	for _, child := range []string{"0", "1", "2"} {
		ord := taxoReader.GetOrdinalFromPath("Dim", child)
		if ord < 0 {
			t.Errorf("ordinal for Dim/%s not found", child)
		}
		path := taxoReader.GetPathComponents(ord)
		if len(path) < 2 || path[len(path)-1] != child {
			t.Errorf("path for ordinal %d: expected ending with %s, got %v", ord, child, path)
		}
	}
}

func TestOrdinalData_SearchableField(t *testing.T) {
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

	// Index docs with a searchable field plus facets.
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		cat := "cat_A"
		if i%2 == 0 {
			cat = "cat_B"
		}
		contentField, _ := document.NewTextField("content", cat, true)
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

	result, err := ftfc.GetTopChildren(10, "Dim")
	if err != nil {
		t.Fatalf("GetTopChildren: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Value != 10 {
		t.Errorf("total value: want 10, got %d", result.Value)
	}
}

func TestOrdinalData_Reindex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	taxoDir := store.NewByteBuffersDirectory()
	defer taxoDir.Close()

	taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("creating taxonomy writer: %v", err)
	}

	config := facets.NewFacetsConfig()
	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index initial batch.
	for i := 0; i < 3; i++ {
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
	writer.Close()
	taxoWriter.Close()

	// Re-index with a new taxonomy writer using CREATE_OR_APPEND.
	taxoWriter2, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("opening existing taxonomy: %v", err)
	}
	defer taxoWriter2.Close()

	writer2, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating second index writer: %v", err)
	}

	// Add more categories.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		ff := facets.NewFacetField("Dim", "reindexed_"+itoa(i))
		builtDoc, err := config.BuildWithTaxonomy(taxoWriter2, doc, ff)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer2.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer2.Commit(); err != nil {
		t.Fatalf("second writer commit: %v", err)
	}
	if err := taxoWriter2.Commit(); err != nil {
		t.Fatalf("second taxonomy commit: %v", err)
	}

	// Verify the taxonomy grew.
	taxoReader, err := facets.NewDirectoryTaxonomyReaderFromWriter(taxoWriter2)
	if err != nil {
		t.Fatalf("taxonomy reader: %v", err)
	}

	if sz := taxoReader.GetSize(); sz <= 4 {
		t.Errorf("expected grown taxonomy > 4, got %d", sz)
	}
}
