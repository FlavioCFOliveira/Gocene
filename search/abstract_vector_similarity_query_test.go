// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/AbstractVectorSimilarityQuery.java
//
// No dedicated Java test peer; tests cover the exported Go contract:
// constructor validation, equals/hash helpers, VisitVectorSimilarityQuery.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// minimalVSQImpl is the smallest valid VectorSimilarityQueryImpl for tests.
// It embeds EmptyQueryVisitorBase to satisfy QueryVisitor defaults and BaseQuery
// for Query.Clone().
type minimalVSQImpl struct {
	search.BaseQuery
	*search.BaseVectorSimilarityQuery
}

func (m *minimalVSQImpl) CreateVectorScorer(_ *index.LeafReaderContext) (search.VectorScorer, error) {
	return nil, nil
}

func (m *minimalVSQImpl) ApproximateSearch(
	_ *index.LeafReaderContext,
	_ search.AcceptDocs,
	_ int,
	_ knn.KnnCollectorManager,
) (*search.TopDocs, error) {
	return &search.TopDocs{}, nil
}

func (m *minimalVSQImpl) Visit(_ search.QueryVisitor) {}
func (m *minimalVSQImpl) String() string              { return "minimalVSQ" }

// testQueryVisitor wraps EmptyQueryVisitorBase and records VisitLeaf calls.
type testQueryVisitor struct {
	search.EmptyQueryVisitorBase
	acceptFn func(string) bool
	visited  []search.Query
}

func (v *testQueryVisitor) AcceptField(f string) bool {
	if v.acceptFn != nil {
		return v.acceptFn(f)
	}
	return true
}

func (v *testQueryVisitor) VisitLeaf(q search.Query) {
	v.visited = append(v.visited, q)
}

func (v *testQueryVisitor) GetSubVisitor(_ search.Occur, _ search.Query) search.QueryVisitor {
	return v
}

// ─── Constructor validation ───────────────────────────────────────────────────

// TestBaseVectorSimilarityQuery_TraversalGtResult verifies that
// traversalSimilarity > resultSimilarity is rejected.
func TestBaseVectorSimilarityQuery_TraversalGtResult(t *testing.T) {
	_, err := search.NewBaseVectorSimilarityQuery("vec", 0.9, 0.5, nil)
	if err == nil {
		t.Fatal("expected error when traversalSimilarity > resultSimilarity, got nil")
	}
}

// TestBaseVectorSimilarityQuery_EmptyField verifies that an empty field is
// rejected.
func TestBaseVectorSimilarityQuery_EmptyField(t *testing.T) {
	_, err := search.NewBaseVectorSimilarityQuery("", 0.5, 0.9, nil)
	if err == nil {
		t.Fatal("expected error for empty field, got nil")
	}
}

// TestBaseVectorSimilarityQuery_Valid verifies a valid construction stores
// all fields correctly.
func TestBaseVectorSimilarityQuery_Valid(t *testing.T) {
	q, err := search.NewBaseVectorSimilarityQuery("vec", 0.5, 0.9, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Field != "vec" {
		t.Errorf("Field = %q, want %q", q.Field, "vec")
	}
	if q.TraversalSimilarity != 0.5 {
		t.Errorf("TraversalSimilarity = %v, want 0.5", q.TraversalSimilarity)
	}
	if q.ResultSimilarity != 0.9 {
		t.Errorf("ResultSimilarity = %v, want 0.9", q.ResultSimilarity)
	}
	if q.Filter != nil {
		t.Errorf("Filter = %v, want nil", q.Filter)
	}
}

// TestBaseVectorSimilarityQuery_EqualThresholds verifies that equal thresholds
// are valid (traversalSimilarity == resultSimilarity is allowed).
func TestBaseVectorSimilarityQuery_EqualThresholds(t *testing.T) {
	_, err := search.NewBaseVectorSimilarityQuery("vec", 0.7, 0.7, nil)
	if err != nil {
		t.Fatalf("equal thresholds should be valid, got: %v", err)
	}
}

// ─── Equals ───────────────────────────────────────────────────────────────────

// TestVectorSimilarityQueryEquals verifies equality semantics.
func TestVectorSimilarityQueryEquals(t *testing.T) {
	a, _ := search.NewBaseVectorSimilarityQuery("f", 0.5, 0.8, nil)
	b, _ := search.NewBaseVectorSimilarityQuery("f", 0.5, 0.8, nil)
	c, _ := search.NewBaseVectorSimilarityQuery("other", 0.5, 0.8, nil)
	d, _ := search.NewBaseVectorSimilarityQuery("f", 0.4, 0.8, nil)
	e, _ := search.NewBaseVectorSimilarityQuery("f", 0.5, 0.9, nil)

	if !search.VectorSimilarityQueryEquals(a, a) {
		t.Errorf("a.Equals(a) = false (identity)")
	}
	if !search.VectorSimilarityQueryEquals(a, b) {
		t.Errorf("a.Equals(b) = false (same values)")
	}
	if search.VectorSimilarityQueryEquals(a, c) {
		t.Errorf("a.Equals(c) = true (different field)")
	}
	if search.VectorSimilarityQueryEquals(a, d) {
		t.Errorf("a.Equals(d) = true (different traversalSimilarity)")
	}
	if search.VectorSimilarityQueryEquals(a, e) {
		t.Errorf("a.Equals(e) = true (different resultSimilarity)")
	}
}

// ─── HashCode ─────────────────────────────────────────────────────────────────

// TestVectorSimilarityQueryHashCode verifies equal queries produce equal hashes.
func TestVectorSimilarityQueryHashCode(t *testing.T) {
	a, _ := search.NewBaseVectorSimilarityQuery("f", 0.5, 0.8, nil)
	b, _ := search.NewBaseVectorSimilarityQuery("f", 0.5, 0.8, nil)
	if search.VectorSimilarityQueryHashCode(a) != search.VectorSimilarityQueryHashCode(b) {
		t.Errorf("equal queries produced different hash codes")
	}
}

// ─── VisitVectorSimilarityQuery ───────────────────────────────────────────────

// TestVisitVectorSimilarityQuery_AcceptedField verifies VisitLeaf is called
// when the visitor accepts the field.
func TestVisitVectorSimilarityQuery_AcceptedField(t *testing.T) {
	q, _ := search.NewBaseVectorSimilarityQuery("vec", 0.5, 0.8, nil)
	impl := &minimalVSQImpl{BaseVectorSimilarityQuery: q}

	v := &testQueryVisitor{
		acceptFn: func(f string) bool { return f == "vec" },
	}
	search.VisitVectorSimilarityQuery(impl, "vec", v)
	if len(v.visited) != 1 {
		t.Errorf("VisitLeaf call count = %d, want 1", len(v.visited))
	}
}

// TestVisitVectorSimilarityQuery_RejectedField verifies VisitLeaf is NOT called
// when the visitor rejects the field.
func TestVisitVectorSimilarityQuery_RejectedField(t *testing.T) {
	q, _ := search.NewBaseVectorSimilarityQuery("vec", 0.5, 0.8, nil)
	impl := &minimalVSQImpl{BaseVectorSimilarityQuery: q}

	v := &testQueryVisitor{
		acceptFn: func(_ string) bool { return false },
	}
	search.VisitVectorSimilarityQuery(impl, "vec", v)
	if len(v.visited) != 0 {
		t.Errorf("VisitLeaf was called when field was rejected (count=%d)", len(v.visited))
	}

// ─── CreateVectorSimilarityWeight ────────────────────────────────────────────

// TestCreateVectorSimilarityWeight_NoFilter verifies that a weight can be
// constructed without a filter (filterWeight = nil).
func TestCreateVectorSimilarityWeight_NoFilter(t *testing.T) {
	q, _ := search.NewBaseVectorSimilarityQuery("vec", 0.5, 0.8, nil)
	impl := &minimalVSQImpl{BaseVectorSimilarityQuery: q}

	w, err := search.CreateVectorSimilarityWeight(impl, q, nil, 1.0)
	if err != nil {
		t.Fatalf("CreateVectorSimilarityWeight: %v", err)
	}
	if w == nil {
		t.Fatal("CreateVectorSimilarityWeight returned nil")
	}
}