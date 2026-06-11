// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestDocValuesCompression_ModeValues verifies the compression mode enum values.
func TestDocValuesCompression_ModeValues(t *testing.T) {
	if Lucene80DVModeBestSpeed != 0 {
		t.Errorf("Lucene80DVModeBestSpeed = %d, want 0", Lucene80DVModeBestSpeed)
	}
	if Lucene80DVModeBestCompression != 1 {
		t.Errorf("Lucene80DVModeBestCompression = %d, want 1", Lucene80DVModeBestCompression)
	}
}

// TestDocValuesCompression_ModeCompare verifies that both modes produce the
// same Name() and differ only in Mode().
func TestDocValuesCompression_ModeCompare(t *testing.T) {
	speed := NewLucene80DocValuesFormat()
	comp := NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestCompression)

	if speed.Name() != comp.Name() {
		t.Errorf("speed Name=%q, comp Name=%q, want equal", speed.Name(), comp.Name())
	}
	if speed.Mode() == comp.Mode() {
		t.Error("expected different modes, got equal")
	}
}

// TestDocValuesCompression_DefaultIsSpeed verifies the default mode.
func TestDocValuesCompression_DefaultIsSpeed(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if f.Mode() != Lucene80DVModeBestSpeed {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestSpeed", f.Mode())
	}
}
