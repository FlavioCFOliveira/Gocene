// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"testing"
)

// TestRWCodec_Name verifies the codec name matches the Java original.
//
// In the Java test tree, Lucene95RWCodec is a test-support class that exposes
// a writable version of Lucene95Codec. In Gocene the production codec is
// always read-only; this test validates the codec that the RW variant would wrap.
func TestRWCodec_Name(t *testing.T) {
	c := NewLucene95Codec()
	if c.Name() != "Lucene95" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene95")
	}
}

// TestRWCodec_BestCompressionMode verifies BEST_COMPRESSION mode construction.
func TestRWCodec_BestCompressionMode(t *testing.T) {
	c := NewLucene95CodecWithMode(Lucene95StoredFieldsBestCompression)
	if c == nil {
		t.Fatal("expected non-nil codec")
	}
	if c.Name() != "Lucene95" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene95")
	}
	if c.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat is nil")
	}
}

// TestRWCodec_KnnVectorsFormatNotNil verifies the KNN vectors format is non-nil.
func TestRWCodec_KnnVectorsFormatNotNil(t *testing.T) {
	c := NewLucene95Codec()
	knn := c.KnnVectorsFormat()
	if knn == nil {
		t.Fatal("KnnVectorsFormat is nil")
	}
	if knn.Name() != "Lucene95HnswVectorsFormat" {
		t.Errorf("KnnVectorsFormat.Name: got %q, want %q", knn.Name(), "Lucene95HnswVectorsFormat")
	}
}

// TestRWCodec_GetKnnVectorsFormatForField verifies per-field KNN resolution.
func TestRWCodec_GetKnnVectorsFormatForField(t *testing.T) {
	c := NewLucene95Codec()
	knn := c.GetKnnVectorsFormatForField("any_field")
	if knn == nil {
		t.Fatal("GetKnnVectorsFormatForField returned nil")
	}
}

// TestRWCodec_StoredFieldsModeConstants verifies the mode enum values.
func TestRWCodec_StoredFieldsModeConstants(t *testing.T) {
	if Lucene95StoredFieldsBestSpeed == Lucene95StoredFieldsBestCompression {
		t.Error("BEST_SPEED and BEST_COMPRESSION must be distinct values")
	}
}
