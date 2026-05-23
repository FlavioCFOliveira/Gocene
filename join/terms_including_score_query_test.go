// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestTermsIncludingScoreQuery_Construction(t *testing.T) {
	terms := util.NewBytesRefHash()
	_, _ = terms.Add(util.NewBytesRef([]byte("apple")))
	_, _ = terms.Add(util.NewBytesRef([]byte("banana")))

	scores := []float32{1.0, 2.0}
	q := NewTermsIncludingScoreQuery(Max, "toField", false, terms, scores, "fromField", nil, nil)
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if q.GetToField() != "toField" {
		t.Errorf("GetToField() = %q, want %q", q.GetToField(), "toField")
	}
	if q.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", q.GetScoreMode())
	}
	if q.IsMultipleValuesPerDocument() {
		t.Error("IsMultipleValuesPerDocument() = true, want false")
	}
}

func TestTermsIncludingScoreQuery_String(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Min, "f", false, terms, nil, "src", nil, nil)
	s := q.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestTermsIncludingScoreQuery_Equals(t *testing.T) {
	terms := util.NewBytesRefHash()
	id := "ctx1"
	q1 := NewTermsIncludingScoreQuery(Max, "toF", false, terms, nil, "fromF", nil, id)
	q2 := NewTermsIncludingScoreQuery(Max, "toF", false, terms, nil, "fromF", nil, id)
	q3 := NewTermsIncludingScoreQuery(Min, "toF", false, terms, nil, "fromF", nil, id)

	if !q1.Equals(q2) {
		t.Error("q1.Equals(q2) = false, want true")
	}
	if q1.Equals(q3) {
		t.Error("q1.Equals(q3) = true, want false (different scoreMode)")
	}
}

func TestTermsIncludingScoreQuery_Clone(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", true, terms, nil, "src", nil, nil)
	clone := q.Clone()
	if clone == q {
		t.Error("Clone() returned same pointer")
	}
	if !q.Equals(clone) {
		t.Error("original and clone are not equal")
	}
}

func TestTermsIncludingScoreQuery_CreateWeight(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", false, terms, nil, "src", nil, nil)
	w, err := q.CreateWeight(nil, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil weight")
	}
	if w.GetQuery() != q {
		t.Error("weight.GetQuery() != original query")
	}
}

func TestTermsIncludingScoreQuery_Rewrite(t *testing.T) {
	terms := util.NewBytesRefHash()
	q := NewTermsIncludingScoreQuery(Max, "f", false, terms, nil, "src", nil, nil)
	rw, err := q.Rewrite(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rw != q {
		t.Error("Rewrite() should return self")
	}
}
