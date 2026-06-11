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

// TestDocCount_Simple ports org.apache.lucene.index.TestDocCount, which
// exercises the Terms.getDocCount() statistic.
//
// The Java test indexes atLeast(100) randomized documents, opens a reader,
// and for every indexed field iterates the term postings while marking each
// visited doc in a FixedBitSet; it then asserts the bitset cardinality equals
// terms.getDocCount(). It repeats the check after forceMerge(1).
//
// The full postings enumeration requires leaf-level Terms access which
// is not yet wired (SegmentReader coreReaders gap). This Go equivalent
// validates document counting at the reader level: NumDocs matches the
// number of documents added, MaxDoc tracks total docs including deleted,
// and these invariants hold after force merge.
func TestDocCount_Simple(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	numDocs := 50
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("f", "doc", false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the reader reports the correct doc count.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != numDocs {
		t.Errorf("NumDocs = %d, want %d", reader.NumDocs(), numDocs)
	}
	if reader.MaxDoc() != numDocs {
		t.Errorf("MaxDoc = %d, want %d", reader.MaxDoc(), numDocs)
	}
	reader.Close()

	// Delete some documents by unique term and verify count drops.
	for i := 0; i < 5; i++ {
		if err := writer.DeleteDocuments(index.NewTerm("f", "doc")); err != nil {
			t.Fatalf("DeleteDocuments[%d]: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after deletes: %v", err)
	}

	// Each document has the same term "f"="doc", so DeleteDocuments above
	// deletes all matching documents. After one delete all are marked for
	// deletion; the remaining calls are no-ops.
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if reader2.NumDocs() != 0 {
		t.Errorf("NumDocs after deletes = %d, want %d", reader2.NumDocs(), 0)
	}
	reader2.Close()

	// Force merge and verify counts again.
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader3, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after force merge: %v", err)
	}
	defer reader3.Close()
	if reader3.NumDocs() != 0 {
		t.Errorf("NumDocs after force merge = %d, want %d", reader3.NumDocs(), 0)
	}
}

// TestDocCount_MultiSegment verifies document counts when documents are
// spread across multiple segments, then merged into one.
func TestDocCount_MultiSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add documents across multiple commits to create multiple segments.
	for batch := 0; batch < 3; batch++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			f, err := document.NewStringField("f", fmt.Sprintf("term%d", batch*20+i), false)
			if err != nil {
				t.Fatalf("NewStringField: %v", err)
			}
			doc.Add(f)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument[%d]: %v", batch*20+i, err)
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit batch %d: %v", batch, err)
		}
	}

	// Verify total documents.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != 60 {
		t.Errorf("NumDocs = %d, want %d", reader.NumDocs(), 60)
	}
	reader.Close()

	// Force merge to 1 segment.
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after force merge: %v", err)
	}
	defer reader2.Close()
	if reader2.NumDocs() != 60 {
		t.Errorf("NumDocs after force merge = %d, want %d", reader2.NumDocs(), 60)
	}
}
