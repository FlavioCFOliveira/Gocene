// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"math"
	"testing"
)

func makeTestCollector(t *testing.T) *GlobalOrdinalsWithScoreCollector {
	t.Helper()
	c, err := NewGlobalOrdinalsWithScoreCollectorMax("f", nil, 10, 1, math.MaxInt32)
	if err != nil {
		t.Fatalf("collector: %v", err)
	}
	return &c.GlobalOrdinalsWithScoreCollector
}

func TestGlobalOrdinalsWithScoreQuery_Construction(t *testing.T) {
	coll := makeTestCollector(t)
	q := NewGlobalOrdinalsWithScoreQuery(coll, Max, "joinF", nil, nil, nil, 1, math.MaxInt32, "id1")
	if q == nil {
		t.Fatal("expected non-nil query")
	}
	if q.GetJoinField() != "joinF" {
		t.Errorf("GetJoinField() = %q, want %q", q.GetJoinField(), "joinF")
	}
	if q.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", q.GetScoreMode())
	}
}

func TestGlobalOrdinalsWithScoreQuery_String(t *testing.T) {
	coll := makeTestCollector(t)
	q := NewGlobalOrdinalsWithScoreQuery(coll, Max, "f", nil, nil, nil, 1, math.MaxInt32, nil)
	if q.String() == "" {
		t.Error("String() returned empty string")
	}
}

func TestGlobalOrdinalsWithScoreQuery_Equals(t *testing.T) {
	coll := makeTestCollector(t)
	id := "ctx1"
	q1 := NewGlobalOrdinalsWithScoreQuery(coll, Max, "f", nil, nil, nil, 1, math.MaxInt32, id)
	q2 := NewGlobalOrdinalsWithScoreQuery(coll, Max, "f", nil, nil, nil, 1, math.MaxInt32, id)
	q3 := NewGlobalOrdinalsWithScoreQuery(coll, Min, "f", nil, nil, nil, 1, math.MaxInt32, id)

	if !q1.Equals(q2) {
		t.Error("q1.Equals(q2) = false, want true")
	}
	if q1.Equals(q3) {
		t.Error("q1.Equals(q3) = true, want false (different scoreMode)")
	}
}

func TestGlobalOrdinalsWithScoreQuery_CreateWeight(t *testing.T) {
	coll := makeTestCollector(t)
	q := NewGlobalOrdinalsWithScoreQuery(coll, Max, "f", nil, nil, nil, 1, math.MaxInt32, nil)
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

func TestGlobalOrdinalsWithScoreQuery_IsCacheable(t *testing.T) {
	coll := makeTestCollector(t)
	q := NewGlobalOrdinalsWithScoreQuery(coll, Max, "f", nil, nil, nil, 1, math.MaxInt32, nil)
	w, _ := q.CreateWeight(nil, false, 1.0)
	gw := w.(*globalOrdinalsWithScoreWeight)
	if gw.IsCacheable(nil) {
		t.Error("IsCacheable() = true, want false (disabled by design)")
	}
}
