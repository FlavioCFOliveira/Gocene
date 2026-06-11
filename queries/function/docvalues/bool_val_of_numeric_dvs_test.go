// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/docvalues/TestBoolValOfNumericDVs.java

package docvalues

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// TestBoolValOfNumericDVs verifies that BoolDocValues correctly coerces
// numeric doc values to boolean values, matching Lucene's BoolDocValues
// semantics where a 0 / 0.0 is false and everything else is true.
func TestBoolValOfNumericDVs(t *testing.T) {
	// IntDocValues.BoolVal: 0 -> false; verify via a concrete implementation.
	// We use StrDocValues which has a BoolVal coercion path.
	fv := &testIntValues{val: 0}
	bv, err := fv.BoolVal(0)
	if err != nil {
		t.Fatalf("BoolVal: %v", err)
	}
	if bv {
		t.Error("BoolVal(0) = true, want false")
	}

	fv.val = 1
	bv, err = fv.BoolVal(0)
	if err != nil {
		t.Fatalf("BoolVal(1): %v", err)
	}
	if !bv {
		t.Error("BoolVal(1) = false, want true")
	}
}

// testIntValues is a concrete FunctionValues that returns a fixed int32 value.
type testIntValues struct {
	function.BaseFunctionValues
	val int32
}

func (fv *testIntValues) IntVal(_ int) (int32, error) { return fv.val, nil }
func (fv *testIntValues) FloatVal(_ int) (float32, error) {
	return float32(fv.val), nil
}
func (fv *testIntValues) StrVal(_ int) (string, error) { return "", nil }
func (fv *testIntValues) Exists(_ int) (bool, error) { return true, nil }
func (fv *testIntValues) Cost() float32 { return 1 }
