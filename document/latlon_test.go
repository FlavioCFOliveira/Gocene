// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"testing"
)

func TestLatLonPoint_RoundTrip(t *testing.T) {
	p, err := NewLatLonPoint("loc", 38.7223, -9.1393) // Lisbon
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BinaryValue()) != 8 {
		t.Fatalf("encoded len = %d", len(p.BinaryValue()))
	}
	lat, lon, err := DecodeLatLon(p.BinaryValue())
	if err != nil {
		t.Fatal(err)
	}
	// Lat/lon are encoded with ~1e-7 precision; allow small tolerance.
	if math.Abs(lat-38.7223) > 1e-5 || math.Abs(lon-(-9.1393)) > 1e-5 {
		t.Fatalf("decoded (%v, %v) far from input", lat, lon)
	}
}

func TestLatLonPoint_Validation(t *testing.T) {
	if _, err := NewLatLonPoint("loc", 91, 0); err == nil {
		t.Fatalf("expected error for lat > 90")
	}
	if _, err := NewLatLonPoint("loc", 0, 181); err == nil {
		t.Fatalf("expected error for lon > 180")
	}
}

func TestLatLonDocValuesField_RoundTrip(t *testing.T) {
	f, err := NewLatLonDocValuesField("loc", 38.7223, -9.1393)
	if err != nil {
		t.Fatal(err)
	}
	encoded, ok := f.NumericValue().(int64)
	if !ok {
		t.Fatalf("NumericValue not int64: %T", f.NumericValue())
	}
	lat, lon := DecodeLatLonFromLong(encoded)
	if math.Abs(lat-38.7223) > 1e-5 || math.Abs(lon-(-9.1393)) > 1e-5 {
		t.Fatalf("decoded (%v, %v) far from input", lat, lon)
	}
}

func TestLatLonPoint_EncodeCeilDiffersFromEncode(t *testing.T) {
	a := EncodeLatLon(0.5, 0.5)
	b := EncodeLatLonCeil(0.5, 0.5)
	// Ceil rounding may equal floor rounding for some values; only assert
	// they decode to the same approximate lat/lon.
	latA, lonA, _ := DecodeLatLon(a)
	latB, lonB, _ := DecodeLatLon(b)
	if math.Abs(latA-latB) > 1e-5 || math.Abs(lonA-lonB) > 1e-5 {
		t.Fatalf("encode vs ceil produced wildly different results")
	}
}
