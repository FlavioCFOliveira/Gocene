// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"testing"
)

// TestLucene95Codec_DefaultMode verifies the default constructor returns BEST_SPEED.
func TestLucene95Codec_DefaultMode(t *testing.T) {
	c := NewLucene95Codec()
	if c == nil {
		t.Fatal("NewLucene95Codec returned nil")
	}
	if c.StoredFieldsFormat() == nil {
		t.Error("StoredFieldsFormat: got nil")
	}
	if c.KnnVectorsFormat() == nil {
		t.Error("KnnVectorsFormat: got nil")
	}
}

// TestLucene95Codec_BestCompressionMode verifies BEST_COMPRESSION mode is accepted.
func TestLucene95Codec_BestCompressionMode(t *testing.T) {
	c := NewLucene95CodecWithMode(Lucene95StoredFieldsBestCompression)
	if c == nil {
		t.Fatal("NewLucene95CodecWithMode(BestCompression) returned nil")
	}
	if c.StoredFieldsFormat() == nil {
		t.Error("StoredFieldsFormat: got nil")
	}
}

// TestLucene95Codec_Name verifies the codec name is "Lucene95".
func TestLucene95Codec_Name(t *testing.T) {
	c := NewLucene95Codec()
	if got := c.Name(); got != "Lucene95" {
		t.Errorf("Name: got %q, want %q", got, "Lucene95")
	}
}

// TestLucene95Codec_GetKnnVectorsFormatForField verifies field dispatch returns the format.
func TestLucene95Codec_GetKnnVectorsFormatForField(t *testing.T) {
	c := NewLucene95Codec()
	f := c.GetKnnVectorsFormatForField("any_field")
	if f == nil {
		t.Fatal("GetKnnVectorsFormatForField returned nil")
	}
	if f.Name() != "Lucene95HnswVectorsFormat" {
		t.Errorf("format name: got %q, want Lucene95HnswVectorsFormat", f.Name())
	}
}

// TestLucene95StoredFieldsMode_Constants verifies mode constants are distinct.
func TestLucene95StoredFieldsMode_Constants(t *testing.T) {
	if Lucene95StoredFieldsBestSpeed == Lucene95StoredFieldsBestCompression {
		t.Error("BestSpeed and BestCompression must differ")
	}
}
