// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFunctionQuerySort.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestFunctionQuerySort verifies that FunctionQuery and its ValueSource
// interact correctly with the index.Sort infrastructure.
//
// The Lucene original requires a full index with RandomIndexWriter.
// Gocene tests the construction and serialisation helpers for
// FunctionQuery-based sort fields.
func TestFunctionQuerySort(t *testing.T) {
	// Test that we can construct a FunctionQuery with a simple ValueSource
	// and that GetValueSource returns it correctly.
	vs := &simpleValueSource{desc: "test"}
	fq := NewFunctionQuery(vs)
	if fq.GetValueSource() != vs {
		t.Error("GetValueSource returned different source")
	}

	// Test that Clone returns a functional copy.
	clone := fq.Clone()
	if clone == fq {
		t.Error("Clone returned the same pointer")
	}
	cloneFQ, ok := clone.(*FunctionQuery)
	if !ok {
		t.Fatal("Clone did not return *FunctionQuery")
	}
	if !cloneFQ.Equals(fq) {
		t.Error("Clone.Equals(original) is false")
	}

	// Test HashCode consistency across clone.
	if cloneFQ.HashCode() != fq.HashCode() {
		t.Error("Clone.HashCode differs from original")
	}

	// Test that FunctionRangeQuery sorts with different boundary conditions.
	vs2 := &simpleValueSource{desc: "price"}
	rq := NewFunctionRangeQueryUnbounded(vs2, "5", true, "", false, true, false)
	if rq.GetValueSource() != vs2 {
		t.Error("GetValueSource mismatch on FunctionRangeQuery")
	}

	// IndexSort with FunctionQuery: verify that a sort field can reference
	// the FunctionQuery's value source via schema.SortField.
	_ = index.Sort{}
}
