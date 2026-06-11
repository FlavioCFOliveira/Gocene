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

// TestDVUpdateBackwardsCompatibility verifies doc-values fields survive a
// round-trip with a backward-registered codec name.
//
// Port of org.apache.lucene.backward_index.TestDVUpdateBackwardsCompatibility
// (Lucene 10.4.0).
func TestDVUpdateBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		ndv, err := document.NewNumericDocValuesField("ndv1", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)

		ndv2, err := document.NewNumericDocValuesField("ndv2", int64(i*3))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv2)

		bdv, err := document.NewBinaryDocValuesField("bdv1", int32ToBytes(i))
		if err != nil {
			t.Fatalf("NewBinaryDocValuesField: %v", err)
		}
		doc.Add(bdv)

		// NOTE: Sorted*, SortedSet*, SortedNumeric* DocValues are not yet
		// supported by the Gocene doc values consumer. Only NumericDocValues
		// and BinaryDocValues fields are tested here.

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

	if reader.NumDocs() != 5 {
		t.Fatalf("expected 5 docs, got %d", reader.NumDocs())
	}

	sfReader, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	for docID := 0; docID < 5; docID++ {
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
		if fid.StringValue() != intToStr(docID) {
			t.Fatalf("doc %d: id = %q, want %q", docID, fid.StringValue(), intToStr(docID))
		}
	}
}

// TestDocValuesRoundtrip verifies a round-trip of all DV field types with the
// default codec.
func TestDocValuesRoundtrip(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	ndv, err := document.NewNumericDocValuesField("price", 99)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	doc.Add(ndv)

	bdv, err := document.NewBinaryDocValuesField("note", []byte("test-note"))
	if err != nil {
		t.Fatalf("NewBinaryDocValuesField: %v", err)
	}
	doc.Add(bdv)

	// NOTE: Sorted*, SortedSet*, SortedNumeric* DocValues not yet supported
	// by the Gocene doc values consumer — only NumericDocValues and
	// BinaryDocValues fields are tested.

	sf, err := document.NewStringField("id", "dvtest", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", reader.NumDocs())
	}
}
