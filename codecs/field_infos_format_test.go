// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLucene94FieldInfosFormat_ReadWrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	rand.Read(id)

	si := index.NewSegmentInfo("_0", 1, dir)
	si.SetID(id)

	builder := index.NewFieldInfosBuilder()

	// Field 1: Simple indexed field
	fi1 := index.NewFieldInfoBuilder("field1", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetStored(true).
		SetAttribute("foo", "bar").
		Build()
	builder.Add(fi1)

	// Field 2: Doc values and points
	fi2 := index.NewFieldInfoBuilder("field2", 1).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesSkipIndexType(index.DocValuesSkipIndexTypeRange).
		SetPointDimensions(1, 1, 4).
		Build()
	builder.Add(fi2)

	// Field 3: Vectors
	fi3 := index.NewFieldInfoBuilder("field3", 2).
		SetVectorAttributes(128, index.VectorEncodingFloat32, index.VectorSimilarityFunctionCosine).
		Build()
	builder.Add(fi3)

	infos := builder.Build()

	format := codecs.NewLucene94FieldInfosFormat()
	context := store.IOContextWrite

	// Write
	err := format.Write(dir, si, "", infos, context)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read
	infos2, err := format.Read(dir, si, "", store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if infos2.Size() != infos.Size() {
		t.Errorf("Expected size %d, got %d", infos.Size(), infos2.Size())
	}

	// Verify Field 1
	f1_2 := infos2.GetByName("field1")
	if f1_2 == nil {
		t.Fatal("field1 not found")
	}
	if f1_2.IndexOptions() != fi1.IndexOptions() {
		t.Errorf("field1: expected index options %v, got %v", fi1.IndexOptions(), f1_2.IndexOptions())
	}
	if f1_2.GetAttribute("foo") != "bar" {
		t.Errorf("field1: expected attribute foo=bar, got %s", f1_2.GetAttribute("foo"))
	}

	// Verify Field 2
	f2_2 := infos2.GetByName("field2")
	if f2_2 == nil {
		t.Fatal("field2 not found")
	}
	if f2_2.DocValuesType() != fi2.DocValuesType() {
		t.Errorf("field2: expected doc values type %v, got %v", fi2.DocValuesType(), f2_2.DocValuesType())
	}
	if f2_2.DocValuesSkipIndexType() != fi2.DocValuesSkipIndexType() {
		t.Errorf("field2: expected skip index type %v, got %v", fi2.DocValuesSkipIndexType(), f2_2.DocValuesSkipIndexType())
	}
	if f2_2.PointDimensionCount() != fi2.PointDimensionCount() {
		t.Errorf("field2: expected point dim count %d, got %d", fi2.PointDimensionCount(), f2_2.PointDimensionCount())
	}

	// Verify Field 3
	f3_2 := infos2.GetByName("field3")
	if f3_2 == nil {
		t.Fatal("field3 not found")
	}
	if f3_2.VectorDimension() != fi3.VectorDimension() {
		t.Errorf("field3: expected vector dimension %d, got %d", fi3.VectorDimension(), f3_2.VectorDimension())
	}
	if f3_2.VectorEncoding() != fi3.VectorEncoding() {
		t.Errorf("field3: expected vector encoding %v, got %v", fi3.VectorEncoding(), f3_2.VectorEncoding())
	}
	if f3_2.VectorSimilarityFunction() != fi3.VectorSimilarityFunction() {
		t.Errorf("field3: expected vector similarity %v, got %v", fi3.VectorSimilarityFunction(), f3_2.VectorSimilarityFunction())
	}
}
