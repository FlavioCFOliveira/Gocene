// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Test2BDocs validates that postings are correctly written and read back for
// a moderate number of documents, each carrying a unique StringField. This
// is a scaled-down version of Lucene's @Monster Test2BDocs that indexes 2B
// docs and tests random advance through the postings.
func Test2BDocs(t *testing.T) {
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
		sf, err := document.NewStringField("id", "doc_"+strconv.Itoa(i), false)
		if err != nil {
			t.Fatalf("NewStringField(%d): %v", i, err)
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

	terms, err := leaves[0].LeafReader().Terms("id")
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

	seen := 0
	for {
		term, err := termsEnum.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if term == nil {
			break
		}
		docFreq, err := termsEnum.DocFreq()
		if err != nil {
			t.Fatalf("DocFreq: %v", err)
		}
		if docFreq != 1 {
			t.Errorf("term %q: docFreq = %d, want 1", term.Text(), docFreq)
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("iterated %d terms, want %d", seen, numDocs)
	}

	// Test random advance: pick every 50th doc ID and verify postings.
	for i := 0; i < numDocs; i += 50 {
		termText := "doc_" + strconv.Itoa(i)
		ok, err := termsEnum.SeekExact(index.NewTerm("id", termText))
		if err != nil {
			t.Fatalf("SeekExact(%q): %v", termText, err)
		}
		if !ok {
			t.Errorf("SeekExact(%q): not found", termText)
			continue
		}
		postings, err := termsEnum.Postings(0)
		if err != nil {
			t.Fatalf("Postings(%q): %v", termText, err)
		}
		docID, err := postings.NextDoc()
		if err != nil {
			t.Fatalf("Postings.NextDoc(%q): %v", termText, err)
		}
		if docID == index.NO_MORE_DOCS {
			t.Errorf("term %q has no postings", termText)
		}
	}
}
