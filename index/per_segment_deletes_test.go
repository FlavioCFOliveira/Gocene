// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPerSegmentDeletes ports org.apache.lucene.index.TestPerSegmentDeletes.
//
// The Lucene test installs a custom RangeMergePolicy, buffers documents across
// several commits, deletes terms, then drives writer.maybeMerge() to verify
// that per-segment deletes are applied during a merge.
//
// This Go equivalent uses ForceMerge(1) instead of a custom merge policy.
// It adds documents across two commits, deletes a subset, force-merges,
// and asserts the surviving doc count via OpenDirectoryReader.
func TestPerSegmentDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add 5 docs and commit -> segment 1.
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("id", fmt.Sprintf("doc%d", i), false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit1: %v", err)
	}

	// Add 5 more docs and commit -> segment 2.
	for i := 5; i < 10; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("id", fmt.Sprintf("doc%d", i), false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit2: %v", err)
	}

	// Delete a document by term.
	if err := writer.DeleteDocuments(index.NewTerm("id", "doc3")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}

	// Force merge to a single segment, applying deletes.
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Open a reader and verify the doc count: 10 added - 1 deleted = 9.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 9 {
		t.Errorf("NumDocs = %d, want %d", reader.NumDocs(), 9)
	}
	// After ForceMerge, deleted documents are physically removed,
	// so MaxDoc equals NumDocs.
	if reader.MaxDoc() != 9 {
		t.Errorf("MaxDoc = %d, want %d", reader.MaxDoc(), 9)
	}
}
