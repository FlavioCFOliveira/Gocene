// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"
)

func TestIntRange(t *testing.T) {
	f := NewIntRange("myrange", 1, 100)
	if f == nil {
		t.Fatal("NewIntRange returned nil")
	}
	if f.Min() != 1 {
		t.Errorf("Min() = %d, want 1", f.Min())
	}
	if f.Max() != 100 {
		t.Errorf("Max() = %d, want 100", f.Max())
	}
}

func TestFloatRange(t *testing.T) {
	f := NewFloatRange("floatrange", 1.5, 99.5)
	if f == nil {
		t.Fatal("NewFloatRange returned nil")
	}
	if f.Min() != 1.5 {
		t.Errorf("Min() = %v, want 1.5", f.Min())
	}
	if f.Max() != 99.5 {
		t.Errorf("Max() = %v, want 99.5", f.Max())
	}
}

func TestDoubleRange(t *testing.T) {
	f, err := NewDoubleRange("doublerange", []float64{1.5}, []float64{99.5})
	if err != nil {
		t.Fatalf("NewDoubleRange error: %v", err)
	}
	if f == nil {
		t.Fatal("NewDoubleRange returned nil")
	}
}
