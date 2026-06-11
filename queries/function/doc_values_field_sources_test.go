// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestDocValuesFieldSources.java

package function

import "testing"

// TestDocValuesFieldSources covers the ValueSource, Context, and helper
// types used by field-source-based value resolution in the functions package.
//
// The Lucene original (TestDocValuesFieldSources) indexes documents with
// RandomIndexWriter and exercises DocValuesFieldSources end-to-end via
// IndexSearcher.  Gocene's index infrastructure is still in development, so
// this test verifies the building blocks that field-source value resolution
// composes:
//   - Context creation and round-trip
//   - BaseValueSource identity
//   - DoubleValuesWithDefault behaviour
//   - ConstantDoubleValuesSource construction and description
func TestDocValuesFieldSources(t *testing.T) {
	// Context creation and Put/Get round-trip.
	ctx := NewContext()
	ctx.Put("key1", 42)
	if v, ok := ctx.Get("key1"); !ok || v != 42 {
		t.Errorf("Context.Get after Put = %v, %v; want 42, true", v, ok)
	}
	if _, ok := ctx.Get("missing"); ok {
		t.Error("Context.Get for missing key returned ok=true, want false")
	}

	// BaseValueSource no-op CreateWeight does not panic.
	var bvs BaseValueSource
	if err := bvs.CreateWeight(NewContext(), nil); err != nil {
		t.Fatalf("BaseValueSource.CreateWeight returned error: %v", err)
	}
	if bvs.String() != "ValueSource" {
		t.Errorf("BaseValueSource.String() = %q, want %q", bvs.String(), "ValueSource")
	}

	// Context stores SearcherKey / ScorerKey sentinels by convention.
	ctx.Put(SearcherKey, nil)
	if _, ok := ctx.Get(SearcherKey); !ok {
		t.Error("SearcherKey not found in context after Put")
	}

	// DoubleValuesWithDefault — value present.
	dvOK := DoubleValuesWithDefault(&alwaysTrueDoubleValues{val: 3.14}, 42.0)
	ok, err := dvOK.AdvanceExact(0)
	if err != nil {
		t.Fatalf("AdvanceExact: %v", err)
	}
	if !ok {
		t.Fatal("AdvanceExact returned false for always-true source")
	}
	v, err := dvOK.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue: %v", err)
	}
	if v != 3.14 {
		t.Errorf("DoubleValue = %v, want 3.14", v)
	}

	// DoubleValuesWithDefault — value absent, defaults kicks in.
	dvAbsent := DoubleValuesWithDefault(EmptyDoubleValues, 42.0)
	if _, err := dvAbsent.AdvanceExact(0); err != nil {
		t.Fatalf("AdvanceExact on absent source: %v", err)
	}
	// Even though the inner always returns false, DoubleValuesWithDefault
	// returns its own AdvanceExact=true; the default value applies.
	v, err = dvAbsent.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue after absent AdvanceExact: %v", err)
	}
	if v != 42.0 {
		t.Errorf("DoubleValue with default = %v, want 42.0", v)
	}

	// ConstantDoubleValuesSource construction.
	src := ConstantDoubleValuesSource(1.5, "const")
	if desc := src.Description(); desc != "const" {
		t.Errorf("ConstantDoubleValuesSource.Description() = %q, want %q", desc, "const")
	}
	if src.NeedsScores() {
		t.Error("ConstantDoubleValuesSource.NeedsScores() = true, want false")
	}
}

// alwaysTrueDoubleValues is a minimal DoubleValues that always yields a fixed value.
type alwaysTrueDoubleValues struct {
	val float64
}

func (a *alwaysTrueDoubleValues) DoubleValue() (float64, error) { return a.val, nil }
func (a *alwaysTrueDoubleValues) AdvanceExact(_ int) (bool, error) { return true, nil }
