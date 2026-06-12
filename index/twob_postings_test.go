// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Test2BPostings validates that postings (term/document pairs) are correctly
// written and read back at moderate scale. This is a scaled-down version of
// Lucene's @Monster/Nightly Test2BPostings that indexes 82M docs producing
// >2B term/doc pairs.
func Test2BPostings(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetUseCompoundFile(false)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 1000
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("f", "term", false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
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
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatal("no leaves")
	}

	terms, err := leaves[0].LeafReader().Terms("f")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms returned nil")
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	// Only one unique term "term" across docs.
	term, err := termsEnum.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if term == nil {
		t.Fatal("expected at least one term")
	}
	docFreq, err := termsEnum.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	if docFreq != numDocs {
		t.Errorf("docFreq = %d, want %d", docFreq, numDocs)
	}

	// Verify postings enumeration yields all docs.
	postings, err := termsEnum.Postings(0)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	seen := 0
	for {
		docID, err := postings.NextDoc()
		if err != nil {
			t.Fatalf("Postings.NextDoc: %v", err)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("postings count = %d, want %d", seen, numDocs)
	}
}
