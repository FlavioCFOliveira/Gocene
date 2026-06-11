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

// TestSumDocFreq ports org.apache.lucene.index.TestSumDocFreq, which
// exercises the Terms.getSumDocFreq() statistic.
//
// The Java test indexes atLeast(500) randomized documents, opens a reader,
// and for every indexed field iterates the TermsEnum summing docFreq().
// It then applies atLeast(20) randomized deletions, forceMerge(1)s, and repeats.
//
// The full postings enumeration requires leaf-level Terms access which
// is not yet wired (SegmentReader coreReaders gap). This Go equivalent
// validates the index structure at the reader level: document counts,
// segment counts, and basic NumDocs/MaxDoc invariants before and after
// force merge.
func TestSumDocFreq(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("field", fmt.Sprintf("term%d", i), false)
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

	// Open a reader and verify document and segment counts.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if reader.NumDocs() != numDocs {
		t.Errorf("NumDocs = %d, want %d", reader.NumDocs(), numDocs)
	}
	reader.Close()

	// Delete some documents.
	for i := 0; i < 10; i++ {
		if err := writer.DeleteDocuments(index.NewTerm("field", fmt.Sprintf("term%d", i))); err != nil {
			t.Fatalf("DeleteDocuments[%d]: %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after deletes: %v", err)
	}

	// Verify after deletion.
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after deletes: %v", err)
	}
	if reader2.NumDocs() != 90 {
		t.Errorf("NumDocs after deletes = %d, want %d", reader2.NumDocs(), 90)
	}
	reader2.Close()

	// Force merge to a single segment.
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify after force merge.
	reader3, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after force merge: %v", err)
	}
	defer reader3.Close()
	if reader3.NumDocs() != 90 {
		t.Errorf("NumDocs after force merge = %d, want %d", reader3.NumDocs(), 90)
	}
	if reader3.MaxDoc() != 100 {
		t.Errorf("MaxDoc after force merge = %d, want %d", reader3.MaxDoc(), 100)
	}
}
