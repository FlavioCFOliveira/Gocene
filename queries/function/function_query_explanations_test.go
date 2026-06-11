// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFunctionQueryExplanations.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestFunctionQueryExplanations exercises the explanation structure produced
// by FunctionQuery and FunctionScoreQuery.
//
// The Lucene original requires an indexed corpus with RandomIndexWriter.
// Gocene tests the explanation-producing infrastructure: that
// ConstantDoubleValuesSource.Explain returns a well-formed search.Explanation
// and that FunctionQuery/FunctionScoreQuery wire up without panicking.
func TestFunctionQueryExplanations(t *testing.T) {
	// ConstantDoubleValuesSource.Explain returns a value explanation.
	src := ConstantDoubleValuesSource(3.14, "constant(3.14)")
	expl, err := src.Explain(nil, 0, search.NewExplanation(true, 0, "score"))
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if !expl.IsMatch() {
		t.Error("expl.IsMatch = false, want true")
	}
	if expl.GetValue() != 3.14 {
		t.Errorf("expl.Value = %v, want 3.14", expl.GetValue())
	}
	if expl.GetDescription() != "constant(3.14)" {
		t.Errorf("expl.Description = %q, want %q", expl.GetDescription(), "constant(3.14)")
	}

	// FunctionQuery.String does not panic and reflects the underlying ValueSource.
	fq := NewFunctionQuery(&simpleValueSource{desc: "vs"})
	str := fq.String()
	if str != "vs" {
		t.Errorf("FunctionQuery.String() = %q, want %q", str, "vs")
	}

	// FunctionScoreQuery.String does not panic.
	inner := search.NewMatchAllDocsQuery()
	fsq := NewFunctionScoreQuery(inner, src)
	if fsq.String() == "" {
		t.Error("FunctionScoreQuery.String() returned empty")
	}
	if fsq.GetWrappedQuery() != inner {
		t.Error("GetWrappedQuery mismatch")
	}
	if fsq.GetSource() != src {
		t.Error("GetSource mismatch")
	}

	// FunctionRangeQuery.String does not panic.
	rangeQ := NewFunctionRangeQuery(&simpleValueSource{desc: "field"}, "5", "10", true, false)
	if rangeQ.String() == "" {
		t.Error("FunctionRangeQuery.String() returned empty")
	}
}

// simpleValueSource is a minimal ValueSource for unit-testing FunctionQuery.
type simpleValueSource struct {
	BaseValueSource
	desc string
}

func (s *simpleValueSource) GetValues(_ Context, _ *index.LeafReaderContext) (FunctionValues, error) {
	return nil, nil
}
func (s *simpleValueSource) Description() string { return s.desc }
func (s *simpleValueSource) Equals(other ValueSource) bool {
	o, ok := other.(*simpleValueSource)
	return ok && o != nil && s.desc == o.desc
}
func (s *simpleValueSource) HashCode() int32 { return hashString(s.desc) }
