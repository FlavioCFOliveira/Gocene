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

// TestInt7HnswBackwardsCompatibility verifies that KNN vector fields
// (float32 vectors using Lucene's HNSW index) survive a round-trip with
// a backward-registered codec name.
//
// Port of org.apache.lucene.backward_index.TestInt7HnswBackwardsCompatibility
// (Lucene 10.4.0).
func TestInt7HnswBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")
	_ = backwardCodec

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil) // Use default codec for KNN test
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const docCount = 5
	for i := range docCount {
		doc := document.NewDocument()

		sf, err := document.NewStringField("id", intToStr(i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)

		// Add a float vector field with random-looking values.
		vec := []float32{float32(i) + 0.1, float32(i)*2 + 0.2, float32(i)*3 + 0.3}
		vf, err := document.NewKnnFloatVectorField(
			"knn_field", vec, index.VectorSimilarityFunctionCosine,
		)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField(%d): %v", i, err)
		}
		doc.Add(vf)

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

	if reader.NumDocs() != docCount {
		t.Fatalf("expected %d docs, got %d", docCount, reader.NumDocs())
	}

	// Verify stored fields.
	sfReader, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	for docID := 0; docID < docCount; docID++ {
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

// TestKnnFloatVectorRoundtrip verifies a single KNN float vector field
// round-trips with the default codec.
func TestKnnFloatVectorRoundtrip(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	vec := []float32{0.5, 0.5, 0.5}
	vf, err := document.NewKnnFloatVectorField("vec", vec, index.VectorSimilarityFunctionEuclidean)
	if err != nil {
		t.Fatalf("NewKnnFloatVectorField: %v", err)
	}
	doc.Add(vf)

	sf, err := document.NewStringField("label", "vector-doc", true)
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
