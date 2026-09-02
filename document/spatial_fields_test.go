// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"testing"
)

// These tests exercise the spatial and feature field types end to end at the
// encoding level. Exhaustive round-trip coverage lives in latlon_test.go,
// xy_test.go and feature_late_interaction_test.go; the tests here assert the
// public constructors are wired and that each field carries the expected
// Lucene-compatible shape (dimension count, bytes-per-dimension, doc-values
// type), so the package's "is this implemented?" smoke checks stay green.

// TestLatLonPoint verifies that NewLatLonPoint builds an 8-byte,
// 2-dimension × 4-byte indexed point and that the value round-trips through
// the GeoEncodingUtils quantization.
func TestLatLonPoint(t *testing.T) {
	const lat, lon = 18.313694, -65.227444
	f, err := NewLatLonPoint("location", lat, lon)
	if err != nil {
		t.Fatalf("NewLatLonPoint: %v", err)
	}
	if got := LatLonPointType.PointDimensionCount(); got != 2 {
		t.Errorf("dimension count = %d, want 2", got)
	}
	if got := LatLonPointType.PointNumBytes(); got != 4 {
		t.Errorf("bytes per dimension = %d, want 4", got)
	}
	encoded := f.BinaryValue()
	if len(encoded) != 8 {
		t.Fatalf("encoded length = %d, want 8", len(encoded))
	}
	gotLat, gotLon, err := DecodeLatLon(encoded)
	if err != nil {
		t.Fatalf("DecodeLatLon: %v", err)
	}
	// The encoding is lossy (quantized); the decoded value must be within
	// one quantization step of the input.
	if math.Abs(gotLat-lat) > 1e-6 || math.Abs(gotLon-lon) > 1e-6 {
		t.Errorf("round-trip = (%v, %v), want ~(%v, %v)", gotLat, gotLon, lat, lon)
	}
}

// TestXYPoint verifies that NewXYPointField builds an 8-byte,
// 2-dimension × 4-byte indexed point and that the value round-trips through
// the XYEncodingUtils quantization.
func TestXYPoint(t *testing.T) {
	const x, y = float32(12.5), float32(-7.25)
	f, err := NewXYPointField("xy", x, y)
	if err != nil {
		t.Fatalf("NewXYPointField: %v", err)
	}
	if got := XYPointFieldType.PointDimensionCount(); got != 2 {
		t.Errorf("dimension count = %d, want 2", got)
	}
	if got := XYPointFieldType.PointNumBytes(); got != 4 {
		t.Errorf("bytes per dimension = %d, want 4", got)
	}
	encoded := f.BinaryValue()
	if len(encoded) != 8 {
		t.Fatalf("encoded length = %d, want 8", len(encoded))
	}
	gotX, gotY, err := DecodeXY(encoded)
	if err != nil {
		t.Fatalf("DecodeXY: %v", err)
	}
	if gotX != x || gotY != y {
		t.Errorf("round-trip = (%v, %v), want (%v, %v)", gotX, gotY, x, y)
	}
}

// TestFeatureField verifies that NewFeatureField builds a feature field
// carrying the supplied feature name and value.
func TestFeatureField(t *testing.T) {
	f, err := NewFeatureField("features", "pagerank", 4.2)
	if err != nil {
		t.Fatalf("NewFeatureField: %v", err)
	}
	if f.Name() != "features" {
		t.Errorf("field name = %q, want \"features\"", f.Name())
	}
}
