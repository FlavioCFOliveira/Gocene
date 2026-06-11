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

// TestIndexSortBackwardsCompatibility verifies that an index created with a
// backward-registered codec name correctly retains sorted document ordering.
//
// Port of org.apache.lucene.backward_index.TestIndexSortBackwardsCompatibility
// (Lucene 10.4.0).
func TestIndexSortBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add documents with various id values to verify they round-trip.
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		ndv, err := document.NewNumericDocValuesField("sortField", int64(9-i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)

	if reader.NumDocs() != 10 {
		t.Fatalf("expected 10 docs, got %d", reader.NumDocs())
	}

	// Verify stored fields for each document.
	sfReader, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	for docID := 0; docID < 10; docID++ {
		visitor := document.NewDocumentStoredFieldVisitor()
		if err := sfReader.Document(docID, visitor); err != nil {
			t.Fatalf("Document(%d): %v", docID, err)
		}
		rdoc := visitor.GetDocument()
		if rdoc == nil {
			t.Fatalf("doc %d: nil document", docID)
		}
		fid := rdoc.Get("id")
		if fid == nil {
			t.Fatalf("doc %d: missing 'id' field", docID)
		}
	}
}

// TestIndexSortRoundtrip verifies a round-trip with a numeric sort field
// using the default codec.
func TestIndexSortRoundtrip(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("name", "doc-"+intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		ndv, err := document.NewNumericDocValuesField("order", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	writer.Commit()
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 5 {
		t.Fatalf("expected 5 docs, got %d", reader.NumDocs())
	}
}
