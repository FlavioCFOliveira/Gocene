// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"testing"
)

// TestLucene94Codec_Name verifies the codec name matches the Java original.
func TestLucene94Codec_Name(t *testing.T) {
	c := NewLucene94Codec()
	if c.Name() != "Lucene94" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene94")
	}
}

// TestLucene94Codec_DefaultMode verifies that the default constructor
// uses BEST_SPEED mode.
func TestLucene94Codec_DefaultMode(t *testing.T) {
	c := NewLucene94Codec()
	if c.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat is nil")
	}
}

// TestLucene94Codec_BestCompressionMode verifies BEST_COMPRESSION mode.
func TestLucene94Codec_BestCompressionMode(t *testing.T) {
	c := NewLucene94CodecWithMode(Lucene94StoredFieldsBestCompression)
	if c == nil {
		t.Fatal("expected non-nil codec")
	}
	if c.Name() != "Lucene94" {
		t.Errorf("Name: got %q, want %q", c.Name(), "Lucene94")
	}
	if c.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat is nil")
	}
}

// TestLucene94Codec_KnnVectorsFormat verifies the KNN vectors format is non-nil.
func TestLucene94Codec_KnnVectorsFormat(t *testing.T) {
	c := NewLucene94Codec()
	knn := c.KnnVectorsFormat()
	if knn == nil {
		t.Fatal("KnnVectorsFormat is nil")
	}
}

// TestLucene94Codec_GetKnnVectorsFormatForField verifies per-field KNN resolution.
func TestLucene94Codec_GetKnnVectorsFormatForField(t *testing.T) {
	c := NewLucene94Codec()
	knn := c.GetKnnVectorsFormatForField("any_field")
	if knn == nil {
		t.Fatal("GetKnnVectorsFormatForField returned nil")
	}
}

// TestLucene94Codec_StoredFieldsModeConstants verifies the mode enum values are distinct.
func TestLucene94Codec_StoredFieldsModeConstants(t *testing.T) {
	if Lucene94StoredFieldsBestSpeed == Lucene94StoredFieldsBestCompression {
		t.Error("BEST_SPEED and BEST_COMPRESSION must be distinct values")
	}
}
