// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"testing"
)

// TestRWCodec_Name verifies the codec name matches the Java original.
//
// In the Java test tree, Lucene92RWCodec is a test-support class that exposes
// a writable version of Lucene92Codec. In Gocene the production codec is
// always read-only; this test validates the codec that the RW variant would wrap.
func TestRWCodec_Name(t *testing.T) {
	c := NewLucene92Codec()
	if c.Name() != "Lucene92" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene92")
	}
}

// TestRWCodec_BestCompressionMode verifies BEST_COMPRESSION mode construction.
func TestRWCodec_BestCompressionMode(t *testing.T) {
	c := NewLucene92CodecWithMode(Lucene92StoredFieldsBestCompression)
	if c == nil {
		t.Fatal("expected non-nil codec")
	}
	if c.Name() != "Lucene92" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene92")
	}
	if c.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat is nil")
	}
}

// TestRWCodec_KnnVectorsFormat verifies the KNN vectors format is non-nil.
func TestRWCodec_KnnVectorsFormat(t *testing.T) {
	c := NewLucene92Codec()
	knn := c.KnnVectorsFormat()
	if knn == nil {
		t.Fatal("KnnVectorsFormat is nil")
	}
	if knn.Name() != "lucene92HnswVectorsFormat" {
		t.Errorf("KnnVectorsFormat.Name: got %q, want %q", knn.Name(), "lucene92HnswVectorsFormat")
	}
}

// TestRWCodec_GetKnnVectorsFormatForField verifies per-field KNN resolution.
func TestRWCodec_GetKnnVectorsFormatForField(t *testing.T) {
	c := NewLucene92Codec()
	knn := c.GetKnnVectorsFormatForField("any_field")
	if knn == nil {
		t.Fatal("GetKnnVectorsFormatForField returned nil")
	}
}

// TestRWCodec_StoredFieldsModeConstants verifies the mode enum values.
func TestRWCodec_StoredFieldsModeConstants(t *testing.T) {
	if Lucene92StoredFieldsBestSpeed == Lucene92StoredFieldsBestCompression {
		t.Error("BEST_SPEED and BEST_COMPRESSION must be distinct values")
	}
}
