// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestOrdinalData ports the test assertions from
// org.apache.lucene.facet.taxonomy.TestOrdinalData.
//
// Full integration tests (testDocValue, testSearchableField, testReindex)
// require IndexWriter + NumericDocValuesField + DirectoryTaxonomyReader
// and are deferred with t.Skip.
// Unit tests cover ReindexingEnrichedDirectoryTaxonomyWriter metadata API.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
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

// -- Integration stubs -------------------------------------------------------

func TestOrdinalData_DocValue(t *testing.T) {
	t.Skip("requires IndexWriter + NumericDocValuesField + DirectoryTaxonomyReader pipeline")
}

func TestOrdinalData_SearchableField(t *testing.T) {
	t.Skip("requires IndexWriter + DirectoryTaxonomyReader + IndexSearcher pipeline")
}

func TestOrdinalData_Reindex(t *testing.T) {
	t.Skip("requires full reindex pipeline with ReindexingEnrichedDirectoryTaxonomyWriter")
}
