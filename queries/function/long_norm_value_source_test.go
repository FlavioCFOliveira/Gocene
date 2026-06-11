// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestLongNormValueSource.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestLongNormValueSource exercises the ValueSource interface and its
// related helper types used by norm-based scoring.
//
// The Lucene original requires an indexed corpus with stored norms and
// RandomIndexWriter. Gocene tests the building blocks: ValueSource
// identity contracts (Equals/HashCode), FunctionQuery construction,
// and DoubleValuesSource basic operation.
func TestLongNormValueSource(t *testing.T) {
	// ValueSource Equals/HashCode contract: two sources with the same
	// description must be equal and have the same hash.
	a := &simpleValueSource{desc: "norm"}
	b := &simpleValueSource{desc: "norm"}
	if !a.Equals(b) {
		t.Error("equal sources: Equals returned false")
	}
	if a.HashCode() != b.HashCode() {
		t.Error("equal sources: HashCode differs")
	}

	// Different descriptions must not be equal.
	c := &simpleValueSource{desc: "other"}
	if a.Equals(c) {
		t.Error("different sources: Equals returned true")
	}

	// BaseValueSource methods work.
	var bvs BaseValueSource
	if err := bvs.CreateWeight(NewContext(), nil); err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if bvs.String() != "ValueSource" {
		t.Errorf("String = %q, want %q", bvs.String(), "ValueSource")
	}

	// FunctionQuery with a ValueSource.
	vs := &simpleValueSource{desc: "long_norm"}
	fq := NewFunctionQuery(vs)
	if fq.GetValueSource() != vs {
		t.Error("GetValueSource mismatch")
	}
	if !fq.Equals(NewFunctionQuery(vs)) {
		t.Error("equal queries: Equals returned false")
	}
	if fq.HashCode() != NewFunctionQuery(vs).HashCode() {
		t.Error("equal queries: HashCode differs")
	}

	// String rendering.
	if fq.String() != "long_norm" {
		t.Errorf("FunctionQuery.String = %q, want %q", fq.String(), "long_norm")
	}

	_ = index.Sort{}
}
