// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// stubQuery is a minimal Query used to satisfy ConstantScoreWeight's
// non-nil-query contract in the tests below. It carries no real
// behaviour; HashCode is fixed so tests can assert against it.
type stubQuery struct {
	*BaseQuery
	hash int
}

func newStubQuery(hash int) *stubQuery {
	return &stubQuery{BaseQuery: &BaseQuery{}, hash: hash}
}

func (q *stubQuery) HashCode() int { return q.hash }
func (q *stubQuery) Equals(other Query) bool {
	o, ok := other.(*stubQuery)
	return ok && o.hash == q.hash
}
func (q *stubQuery) Clone() Query { return &stubQuery{BaseQuery: &BaseQuery{}, hash: q.hash} }

// TestConstantScoreWeight_NilQueryPanics confirms the constructor
// rejects a nil query with a panic carrying the documented message
// (matches the Java reference's NullPointerException semantics).
func TestConstantScoreWeight_NilQueryPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on nil query")
		}
		if msg, ok := r.(string); !ok || msg == "" {
			t.Fatalf("panic value: got %v, want non-empty string", r)
		}
	}()
	_ = NewConstantScoreWeight(nil, 1.0, nil, nil)
}

// TestConstantScoreWeight_GetQuery_ReturnsConstructorArg confirms
// GetQuery returns the exact query passed to the constructor.
func TestConstantScoreWeight_GetQuery_ReturnsConstructorArg(t *testing.T) {
	t.Parallel()
	q := newStubQuery(7)
	w := NewConstantScoreWeight(q, 0.5, nil, nil)
	if w.GetQuery() != q {
		t.Fatalf("GetQuery: got %v, want %v", w.GetQuery(), q)
	}
}

// TestConstantScoreWeight_Score_ReturnsConstructorScore confirms
// Score returns the boost the weight was built with.
func TestConstantScoreWeight_Score_ReturnsConstructorScore(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(newStubQuery(0), 0.875, nil, nil)
	if got, want := w.Score(), float32(0.875); got != want {
		t.Fatalf("Score: got %v, want %v", got, want)
	}
}

// TestConstantScoreWeight_ScorerSupplier_NilHookReturnsNil confirms
// the default supplier hook returns nil ScorerSupplier (matches the
// Java fast path for fields absent from a leaf).
func TestConstantScoreWeight_ScorerSupplier_NilHookReturnsNil(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(newStubQuery(0), 1.0, nil, nil)
	ctx := index.NewLeafReaderContext(nil, nil, 0, 0)
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if supplier != nil {
		t.Fatalf("ScorerSupplier: got %v, want nil", supplier)
	}
}

// TestConstantScoreWeight_ScorerSupplier_DelegatesToHook confirms a
// non-nil hook is called with the per-leaf context and the result
// is returned verbatim.
func TestConstantScoreWeight_ScorerSupplier_DelegatesToHook(t *testing.T) {
	t.Parallel()
	want := NewConstantScoreScorerSupplierFromIterator(1.0, COMPLETE, NewEmptyDocIdSetIterator())
	w := NewConstantScoreWeight(
		newStubQuery(0),
		1.0,
		func(_ *index.LeafReaderContext) (ScorerSupplier, error) { return want, nil },
		nil,
	)
	got, err := w.ScorerSupplier(index.NewLeafReaderContext(nil, nil, 0, 0))
	if err != nil {
		t.Fatalf("ScorerSupplier: %v", err)
	}
	if got != want {
		t.Fatalf("ScorerSupplier: got %v, want %v", got, want)
	}
}

// TestConstantScoreWeight_ScorerSupplier_PropagatesHookError
// confirms errors from the hook are surfaced to the caller.
func TestConstantScoreWeight_ScorerSupplier_PropagatesHookError(t *testing.T) {
	t.Parallel()
	target := errors.New("leaf unreadable")
	w := NewConstantScoreWeight(
		newStubQuery(0),
		1.0,
		func(_ *index.LeafReaderContext) (ScorerSupplier, error) { return nil, target },
		nil,
	)
	if _, err := w.ScorerSupplier(index.NewLeafReaderContext(nil, nil, 0, 0)); !errors.Is(err, target) {
		t.Fatalf("ScorerSupplier: got err %v, want wrapped %v", err, target)
	}
}

// TestConstantScoreWeight_IsCacheable_DefaultsTrue confirms the
// default cacheable hook reports true, matching the Java reference.
func TestConstantScoreWeight_IsCacheable_DefaultsTrue(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(newStubQuery(0), 1.0, nil, nil)
	if !w.IsCacheable(index.NewLeafReaderContext(nil, nil, 0, 0)) {
		t.Fatalf("IsCacheable default: got false, want true")
	}
}

// TestConstantScoreWeight_IsCacheable_RoutesThroughHook confirms a
// custom cacheable hook overrides the default.
func TestConstantScoreWeight_IsCacheable_RoutesThroughHook(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(
		newStubQuery(0),
		1.0,
		nil,
		func(_ *index.LeafReaderContext) bool { return false },
	)
	if w.IsCacheable(index.NewLeafReaderContext(nil, nil, 0, 0)) {
		t.Fatalf("IsCacheable with hook: got true, want false")
	}
}

// TestConstantScoreWeight_Count_DefaultsMinusOne confirms Count
// reports -1, the Java sentinel for "no sub-linear count".
func TestConstantScoreWeight_Count_DefaultsMinusOne(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(newStubQuery(0), 1.0, nil, nil)
	got, err := w.Count(index.NewLeafReaderContext(nil, nil, 0, 0))
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != -1 {
		t.Fatalf("Count: got %d, want -1", got)
	}

// TestConstantScoreWeight_Matches_ReturnsNil confirms Matches
// reports nil for the constant-score variant.
func TestConstantScoreWeight_Matches_ReturnsNil(t *testing.T) {
	t.Parallel()
	w := NewConstantScoreWeight(newStubQuery(0), 1.0, nil, nil)
	got, err := w.Matches(index.NewLeafReaderContext(nil, nil, 0, 0), 0)
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if got != nil {
		t.Fatalf("Matches: got %v, want nil", got)
	}
}