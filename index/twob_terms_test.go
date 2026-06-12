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

// Test2BTerms validates that term enumeration through the full IndexWriter
// pipeline correctly returns all unique terms. This is a scaled-down version
// of Lucene's @Monster Test2BTerms that indexes >2B unique terms.
func Test2BTerms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetUseCompoundFile(false)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numTerms = 500
	for i := 0; i < numTerms; i++ {
		doc := document.NewDocument()
		// Each document gets a unique term "t_<i>" plus a shared term "common".
		sf1, err := document.NewStringField("f", "t_"+strconv.Itoa(i), false)
		if err != nil {
			t.Fatalf("NewStringField(%d): %v", i, err)
		}
		doc.Add(sf1)
		sf2, err := document.NewStringField("f", "common", false)
		if err != nil {
			t.Fatalf("NewStringField(common): %v", err)
		}
		doc.Add(sf2)
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

	// Verify term count: numTerms unique terms + 1 "common" term.
	wantTermCount := int64(numTerms + 1)
	if got := terms.Size(); got != wantTermCount {
		t.Errorf("Size = %d, want %d", got, wantTermCount)
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
		seen++
	}
	if seen != int(wantTermCount) {
		t.Errorf("iterated %d terms, want %d", seen, wantTermCount)
	}

	// Verify "common" has docFreq = numTerms (appears in every doc).
	ok, err := termsEnum.SeekExact(index.NewTerm("f", "common"))
	if err != nil {
		t.Fatalf("SeekExact(common): %v", err)
	}
	if !ok {
		t.Fatal("term 'common' not found")
	}
	docFreq, err := termsEnum.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	if docFreq != numTerms {
		t.Errorf("common docFreq = %d, want %d", docFreq, numTerms)
	}
}
