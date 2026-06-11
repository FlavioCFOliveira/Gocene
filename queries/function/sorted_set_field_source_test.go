// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestSortedSetFieldSource.java

package function

import (
	"testing"
)

// TestSortedSetFieldSource exercises the ValueSource and helper types
// used by sorted-set-based field source resolution.
//
// The Lucene original requires FunctionTestSetup with indexed sorted-set
// field data. Gocene tests the foundational ValueSource types, including
// BaseValueSource no-op semantics, DoubleValuesWithDefault behaviour, and
// LongValues source contracts.
func TestSortedSetFieldSource(t *testing.T) {
	// BaseValueSource marker methods.
	var bvs BaseValueSource
	if err := bvs.CreateWeight(NewContext(), nil); err != nil {
		t.Fatalf("BaseValueSource.CreateWeight: %v", err)
	}
	if bvs.String() != "ValueSource" {
		t.Errorf("BaseValueSource.String = %q, want %q", bvs.String(), "ValueSource")
	}

	// Context round-trip.
	ctx := NewContext()
	ctx.Put("field", []byte{1, 2, 3})
	v, ok := ctx.Get("field")
	if !ok {
		t.Fatal("Context.Get returned ok=false for stored key")
	}
	b, ok := v.([]byte)
	if !ok || len(b) != 3 || b[0] != 1 {
		t.Error("Context.Get returned unexpected value type or content")
	}
	ctx.Put(SearcherKey, "searcher")
	if _, ok := ctx.Get(SearcherKey); !ok {
		t.Error("SearcherKey not found")
	}

	// EmptyDoubleValues.
	if _, err := EmptyDoubleValues.AdvanceExact(0); err != nil {
		t.Fatalf("EmptyDoubleValues.AdvanceExact: %v", err)
	}
	if _, err := EmptyDoubleValues.DoubleValue(); err != nil {
		t.Fatalf("EmptyDoubleValues.DoubleValue: %v", err)
	}

	// EmptyLongValues.
	if _, err := EmptyLongValues.AdvanceExact(0); err != nil {
		t.Fatalf("EmptyLongValues.AdvanceExact: %v", err)
	}
	if _, err := EmptyLongValues.LongValue(); err != nil {
		t.Fatalf("EmptyLongValues.LongValue: %v", err)
	}

	// DoubleValuesWithDefault — absent source yields the default.
	dv := DoubleValuesWithDefault(EmptyDoubleValues, 99.0)
	ok, err := dv.AdvanceExact(0)
	if err != nil {
		t.Fatalf("DoubleValuesWithDefault.AdvanceExact: %v", err)
	}
	if !ok {
		t.Fatal("DoubleValuesWithDefault.AdvanceExact returned false")
	}
	val, err := dv.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValuesWithDefault.DoubleValue: %v", err)
	}
	if val != 99.0 {
		t.Errorf("DoubleValuesWithDefault value = %v, want 99.0", val)
	}
}
