// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.document.TestDoubleRangeField.
package document

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

const doubleRangeFieldName = "rangeField"

// TestDoubleRangeField_IllegalNaNValues mirrors testIllegalNaNValues: NaN in
// the min or max position must produce an error whose message contains the
// expected substring.
func TestDoubleRangeField_IllegalNaNValues(t *testing.T) {
	_, err := document.NewDoubleRange(doubleRangeFieldName, []float64{math.NaN()}, []float64{5})
	if err == nil {
		t.Fatal("expected error for NaN min value")
	}
	if !strings.Contains(err.Error(), "invalid min value") {
		t.Errorf("error message %q should contain \"invalid min value\"", err.Error())
	}

	_, err = document.NewDoubleRange(doubleRangeFieldName, []float64{5}, []float64{math.NaN()})
	if err == nil {
		t.Fatal("expected error for NaN max value")
	}
	if !strings.Contains(err.Error(), "invalid max value") {
		t.Errorf("error message %q should contain \"invalid max value\"", err.Error())
	}
}

// TestDoubleRangeField_UnevenArrays mirrors testUnevenArrays: min/max slices
// of different lengths must be rejected.
func TestDoubleRangeField_UnevenArrays(t *testing.T) {
	_, err := document.NewDoubleRange(doubleRangeFieldName, []float64{5, 6}, []float64{5})
	if err == nil {
		t.Fatal("expected error for uneven min/max arrays")
	}
	if !strings.Contains(err.Error(), "min/max ranges must agree") {
		t.Errorf("error message %q should contain \"min/max ranges must agree\"", err.Error())
	}
}

// TestDoubleRangeField_OversizeDimensions mirrors testOversizeDimensions:
// more than 4 dimensions must be rejected.
func TestDoubleRangeField_OversizeDimensions(t *testing.T) {
	_, err := document.NewDoubleRange(doubleRangeFieldName,
		[]float64{1, 2, 3, 4, 5},
		[]float64{5, 6, 7, 8, 9})
	if err == nil {
		t.Fatal("expected error for >4 dimensions")
	}
	if !strings.Contains(err.Error(), "does not support greater than 4 dimensions") {
		t.Errorf("error message %q should contain \"does not support greater than 4 dimensions\"", err.Error())
	}
}

// TestDoubleRangeField_MinGreaterThanMax mirrors testMinGreaterThanMax:
// per-dimension min > max must be rejected.
func TestDoubleRangeField_MinGreaterThanMax(t *testing.T) {
	_, err := document.NewDoubleRange(doubleRangeFieldName, []float64{3, 4}, []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for min > max")
	}
	if !strings.Contains(err.Error(), "is greater than max value") {
		t.Errorf("error message %q should contain \"is greater than max value\"", err.Error())
	}
}
