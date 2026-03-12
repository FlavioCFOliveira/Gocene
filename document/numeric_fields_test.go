// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"
)

func TestNewFloatField(t *testing.T) {
	f, err := NewFloatField("price", 99.99, true)
	if err != nil {
		t.Fatalf("NewFloatField failed: %v", err)
	}
	if f == nil {
		t.Fatal("NewFloatField returned nil")
	}
	if f.Name() != "price" {
		t.Errorf("Expected name 'price', got %s", f.Name())
	}
	if f.FloatValue() != 99.99 {
		t.Errorf("Expected value 99.99, got %f", f.FloatValue())
	}
}

func TestNewDoubleField(t *testing.T) {
	f, err := NewDoubleField("price", 99.99, true)
	if err != nil {
		t.Fatalf("NewDoubleField failed: %v", err)
	}
	if f == nil {
		t.Fatal("NewDoubleField returned nil")
	}
	if f.Name() != "price" {
		t.Errorf("Expected name 'price', got %s", f.Name())
	}
	if f.DoubleValue() != 99.99 {
		t.Errorf("Expected value 99.99, got %f", f.DoubleValue())
	}
}

func TestEncodeDecodeFloat32(t *testing.T) {
	tests := []float32{0.0, 1.0, -1.0, 3.14159, -3.14159, 1e30, -1e30}

	for _, val := range tests {
		encoded := encodeFloat32(val)
		if len(encoded) != 4 {
			t.Errorf("Expected 4 bytes, got %d", len(encoded))
		}
		decoded := decodeFloat32(encoded)
		// Allow small floating point differences
		if decoded != val {
			t.Errorf("Expected %f, got %f", val, decoded)
		}
	}
}

func TestEncodeDecodeFloat64(t *testing.T) {
	tests := []float64{0.0, 1.0, -1.0, 3.14159265359, -3.14159265359, 1e300, -1e300}

	for _, val := range tests {
		encoded := encodeFloat64(val)
		if len(encoded) != 8 {
			t.Errorf("Expected 8 bytes, got %d", len(encoded))
		}
		decoded := decodeFloat64(encoded)
		// Allow small floating point differences
		if decoded != val {
			t.Errorf("Expected %f, got %f", val, decoded)
		}
	}
}

func TestEncodeFloat32Ordering(t *testing.T) {
	// Test that encoding preserves ordering
	values := []float32{-100.0, -1.0, 0.0, 1.0, 100.0}
	encoded := make([][]byte, len(values))

	for i, v := range values {
		encoded[i] = encodeFloat32(v)
	}

	// Check ordering
	for i := 0; i < len(encoded)-1; i++ {
		if string(encoded[i]) >= string(encoded[i+1]) {
			t.Errorf("Ordering not preserved at index %d", i)
		}
	}
}

func TestEncodeFloat64Ordering(t *testing.T) {
	// Test that encoding preserves ordering
	values := []float64{-100.0, -1.0, 0.0, 1.0, 100.0}
	encoded := make([][]byte, len(values))

	for i, v := range values {
		encoded[i] = encodeFloat64(v)
	}

	// Check ordering
	for i := 0; i < len(encoded)-1; i++ {
		if string(encoded[i]) >= string(encoded[i+1]) {
			t.Errorf("Ordering not preserved at index %d", i)
		}
	}
}

func TestNewFloatPoint(t *testing.T) {
	p, err := NewFloatPoint("lat", 45.5)
	if err != nil {
		t.Fatalf("NewFloatPoint failed: %v", err)
	}
	if p == nil {
		t.Fatal("NewFloatPoint returned nil")
	}
	if p.Name() != "lat" {
		t.Errorf("Expected name 'lat', got %s", p.Name())
	}
}

func TestNewDoublePoint(t *testing.T) {
	p, err := NewDoublePoint("lat", 45.5)
	if err != nil {
		t.Fatalf("NewDoublePoint failed: %v", err)
	}
	if p == nil {
		t.Fatal("NewDoublePoint returned nil")
	}
	if p.Name() != "lat" {
		t.Errorf("Expected name 'lat', got %s", p.Name())
	}
}
