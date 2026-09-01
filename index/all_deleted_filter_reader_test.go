// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestAllDeletedFilterReader verifies that AllDeletedFilterReader makes all documents appear deleted.
// Source: Ported from Apache Lucene's TestAllDeletedFilterReader.
func TestAllDeletedFilterReader(t *testing.T) {
	dir, _, reader := setupSegmentReader(t)
	defer dir.Close()
	defer reader.Close()

	// Create the filter reader
	filterReader := index.NewAllDeletedFilterReader(reader)

	// Verify NumDocs is 0
	if filterReader.NumDocs() != 0 {
		t.Errorf("Expected NumDocs=0, got %d", filterReader.NumDocs())
	}

	// Verify LiveDocs has no bits set
	liveDocs := filterReader.GetLiveDocs()
	if liveDocs == nil {
		t.Fatal("GetLiveDocs should not return nil")
	}
	if liveDocs.Length() != reader.MaxDoc() {
		t.Errorf("Expected LiveDocs length %d, got %d", reader.MaxDoc(), liveDocs.Length())
	}
	for i := 0; i < liveDocs.Length(); i++ {
		if liveDocs.Get(i) {
			t.Errorf("Expected document %d to be deleted, but it is live", i)
		}
	}

	// Verify CacheHelpers
	if filterReader.GetReaderCacheHelper() != nil {
		t.Error("GetReaderCacheHelper should return nil")
	}
	if filterReader.GetCoreCacheHelper() == nil {
		t.Error("GetCoreCacheHelper should not be nil")
	}
}
