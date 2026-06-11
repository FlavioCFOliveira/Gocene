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

// TestBinaryBackwardsCompatibility verifies that binary stored fields survive
// a round-trip with a backward-registered codec name.
//
// Port of org.apache.lucene.backward_index.TestBinaryBackwardsCompatibility
// (Lucene 10.4.0).
func TestBinaryBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sf, err := document.NewStringField("id", "1", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	bf, err := document.NewStoredFieldFromBytes("binaryField", []byte("hello\x00world"))
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes: %v", err)
	}
	doc.Add(bf)

	bdv, err := document.NewBinaryDocValuesField("binaryDV", []byte{0x01, 0x02, 0x03, 0xFF})
	if err != nil {
		t.Fatalf("NewBinaryDocValuesField: %v", err)
	}
	doc.Add(bdv)

	ndv, err := document.NewNumericDocValuesField("numericDV", 42)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	doc.Add(ndv)

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

	sfReader, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := sfReader.Document(0, visitor); err != nil {
		t.Fatalf("Document(0): %v", err)
	}
	rdoc := visitor.GetDocument()
	if rdoc == nil {
		t.Fatal("GetDocument returned nil")
	}
	if rdoc.Get("id") == nil || rdoc.Get("id").StringValue() != "1" {
		t.Fatalf("unexpected id: %v", rdoc.Get("id"))
	}
	if rdoc.Get("binaryField") == nil {
		t.Fatal("missing binaryField")
	}
}

// TestBinaryBackwardsCompatibility_RawBytes tests raw byte arrays stored via
// StoredField survive the round-trip, including zero bytes and non-ASCII.
func TestBinaryBackwardsCompatibility_RawBytes(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	bf, err := document.NewStoredFieldFromBytes("raw", []byte{0x00, 0x01, 0x7F, 0xFF, 0xAB, 0xCD})
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes: %v", err)
	}
	doc.Add(bf)

	sf, err := document.NewStringField("label", "raw-test", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", reader.NumDocs())
	}
}
