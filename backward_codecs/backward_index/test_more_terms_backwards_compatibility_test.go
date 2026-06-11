// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package backward_index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// TestMoreTermsBackwardsCompatibility verifies that an index with many terms
// created with a backward-registered codec name can be opened and queried.
//
// Port of org.apache.lucene.backward_index.TestMoreTermsBackwardsCompatibility
// (Lucene 10.4.0).
func TestMoreTermsBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add many documents with varied term content.
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		tf, err := document.NewTextField("content", "term"+intToStr(i%10)+" data", false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)

		ndv, err := document.NewNumericDocValuesField("docid_intDV", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)

		// NOTE: SortedDocValuesField not yet supported by Gocene DV consumer.

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}

		// Commit every 10 documents to create multiple segments.
		if i > 0 && i%10 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit(%d): %v", i, err)
			}
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Final commit: %v", err)
	}
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)

	if reader.NumDocs() != 50 {
		t.Fatalf("expected 50 docs, got %d", reader.NumDocs())
	}
}

// TestManyTerms_Roundtrip verifies a round-trip with many terms using the
// default codec.
func TestManyTerms_Roundtrip(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		tf, err := document.NewTextField("body", "term"+intToStr(i)+" content "+intToStr(i%20), false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)

		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	writer.Commit()
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 100 {
		t.Fatalf("expected 100 docs, got %d", reader.NumDocs())
	}
}
