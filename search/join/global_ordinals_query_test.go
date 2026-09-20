// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestGlobalOrdinalsQuery_Construction(t *testing.T) {
	bs, _ := util.NewLongBitSet(10)
	q := NewGlobalOrdinalsQuery(bs, "joinF", nil, nil, nil, "ctx1")
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if q.GetJoinField() != "joinF" {
		t.Errorf("GetJoinField() = %q, want %q", q.GetJoinField(), "joinF")
	}
	if q.GetFoundOrds() != bs {
		t.Error("GetFoundOrds() != expected bitset")
	}
}

func TestGlobalOrdinalsQuery_String(t *testing.T) {
	bs, _ := util.NewLongBitSet(1)
	q := NewGlobalOrdinalsQuery(bs, "myJoinField", nil, nil, nil, nil)
	s := q.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
}

func TestGlobalOrdinalsQuery_Equals(t *testing.T) {
	bs, _ := util.NewLongBitSet(5)
	id := "ctxID"
	q1 := NewGlobalOrdinalsQuery(bs, "f", nil, nil, nil, id)
	q2 := NewGlobalOrdinalsQuery(bs, "f", nil, nil, nil, id)
	q3 := NewGlobalOrdinalsQuery(bs, "other", nil, nil, nil, id)

	if !q1.Equals(q2) {
		t.Error("q1.Equals(q2) = false, want true")
	}
	if q1.Equals(q3) {
		t.Error("q1.Equals(q3) = true, want false (different joinField)")
	}
}

func TestGlobalOrdinalsQuery_Clone(t *testing.T) {
	bs, _ := util.NewLongBitSet(3)
	q := NewGlobalOrdinalsQuery(bs, "f", nil, nil, nil, nil)
	clone := q.Clone()
	if clone == q {
		t.Error("Clone() returned same pointer")
	}
}

func TestGlobalOrdinalsQuery_CreateWeight(t *testing.T) {
	bs, _ := util.NewLongBitSet(3)
	q := NewGlobalOrdinalsQuery(bs, "f", nil, nil, nil, nil)
	w, err := q.CreateWeight(nil, false, 1.0)
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

func TestGlobalOrdinalsQuery_IsCacheable(t *testing.T) {
	bs, _ := util.NewLongBitSet(3)
	q := NewGlobalOrdinalsQuery(bs, "f", nil, nil, nil, nil)
	w, _ := q.CreateWeight(nil, false, 1.0)
	gw := w.(*globalOrdinalsQueryWeight)
	if gw.IsCacheable(nil) {
		t.Error("IsCacheable() = true, want false (disabled by design)")
	}
}
