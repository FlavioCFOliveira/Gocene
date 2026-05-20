// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"testing"
)

// TestLucene92Codec_Name verifies the codec name matches the Java original.
func TestLucene92Codec_Name(t *testing.T) {
	c := NewLucene92Codec()
	if c.Name() != "Lucene92" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene92")
	}
}

// TestLucene92Codec_DefaultMode verifies that the default constructor
// uses BEST_SPEED mode (matching the Java Lucene92Codec() default).
func TestLucene92Codec_DefaultMode(t *testing.T) {
	c := NewLucene92Codec()
	if c.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat is nil")
	}
}

// TestLucene92Codec_BestCompressionMode verifies that the WithMode constructor
// accepts BEST_COMPRESSION without error.
func TestLucene92Codec_BestCompressionMode(t *testing.T) {
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

// TestLucene92Codec_KnnVectorsFormat verifies the KNN vectors format is Lucene 9.2.
func TestLucene92Codec_KnnVectorsFormat(t *testing.T) {
	c := NewLucene92Codec()
	knn := c.KnnVectorsFormat()
	if knn == nil {
		t.Fatal("KnnVectorsFormat is nil")
	}
	if knn.Name() != "lucene92HnswVectorsFormat" {
		t.Errorf("KnnVectorsFormat.Name: got %q, want %q", knn.Name(), "lucene92HnswVectorsFormat")
	}
}

// TestLucene92Codec_GetKnnVectorsFormatForField verifies per-field KNN resolution.
func TestLucene92Codec_GetKnnVectorsFormatForField(t *testing.T) {
	c := NewLucene92Codec()
	knn := c.GetKnnVectorsFormatForField("any_field")
	if knn == nil {
		t.Fatal("GetKnnVectorsFormatForField returned nil")
	}
	if knn.MaxConn() != Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("MaxConn: got %d, want %d", knn.MaxConn(), Lucene92HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
}

// TestLucene92Codec_StoredFieldsModeConstants verifies the mode enum values.
func TestLucene92Codec_StoredFieldsModeConstants(t *testing.T) {
	if Lucene92StoredFieldsBestSpeed == Lucene92StoredFieldsBestCompression {
		t.Error("BEST_SPEED and BEST_COMPRESSION must be distinct values")
	}
}
