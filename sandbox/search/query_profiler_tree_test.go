// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerTree tests.
// (No dedicated Java test peer; tests verify observable tree-building behavior.)
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// stubQuery is a minimal search.Query used for profiling tests.
type stubQuery struct {
	search.BaseQuery
	label string
}

func (q *stubQuery) Rewrite(_ search.IndexReader) (search.Query, error) { return q, nil }
func (q *stubQuery) Clone() search.Query                                { return q }
func (q *stubQuery) Equals(other search.Query) bool {
	o, ok := other.(*stubQuery)
	return ok && o.label == q.label
}
func (q *stubQuery) HashCode() int { return len(q.label) }
func (q *stubQuery) CreateWeight(_ *search.IndexSearcher, _ bool, _ float32) (search.Weight, error) {
	return nil, nil
}

// TestQueryProfilerTree_EmptyTreeNoRoots verifies that a fresh tree returns an
// empty root list.
func TestQueryProfilerTree_EmptyTreeNoRoots(t *testing.T) {
	tree := NewQueryProfilerTree()
	results := tree.GetTree()
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestQueryProfilerTree_SingleRootNode verifies that the first query added
// becomes a root node.
func TestQueryProfilerTree_SingleRootNode(t *testing.T) {
	tree := NewQueryProfilerTree()
	q := &stubQuery{label: "root"}

	bd := tree.GetProfileBreakdown(q)
	if bd == nil {
		t.Fatal("expected non-nil breakdown")
	}
	tree.PollLast()

	results := tree.GetTree()
	if len(results) != 1 {
		t.Fatalf("expected 1 root result, got %d", len(results))
	}
}

// TestQueryProfilerTree_ChildNodeAttachedToParent verifies that a query added
// while the stack is non-empty becomes a child of the current parent.
func TestQueryProfilerTree_ChildNodeAttachedToParent(t *testing.T) {
	tree := NewQueryProfilerTree()
	parent := &stubQuery{label: "parent"}
	child := &stubQuery{label: "child"}

	tree.GetProfileBreakdown(parent) // push parent
	tree.GetProfileBreakdown(child)  // push child (child of parent)
	tree.PollLast()                  // pop child
	tree.PollLast()                  // pop parent

	results := tree.GetTree()
	if len(results) != 1 {
		t.Fatalf("expected 1 root, got %d", len(results))
	}
	children := results[0].GetProfiledChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child of root, got %d", len(children))
	}
}

// TestQueryProfilerTree_TwoRootNodes verifies that two separate root queries
// (each added when the stack is empty) produce two root results.
func TestQueryProfilerTree_TwoRootNodes(t *testing.T) {
	tree := NewQueryProfilerTree()
	q1 := &stubQuery{label: "q1"}
	q2 := &stubQuery{label: "q2"}

	tree.GetProfileBreakdown(q1)
	tree.PollLast()
	tree.GetProfileBreakdown(q2)
	tree.PollLast()

	results := tree.GetTree()
	if len(results) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(results))
	}
}

// TestQueryProfilerTree_RewriteTimeAccumulates verifies that rewrite timing
// accumulates correctly across multiple start/stop cycles.
func TestQueryProfilerTree_RewriteTimeAccumulates(t *testing.T) {
	tree := NewQueryProfilerTree()

	tree.StartRewriteTime()
	elapsed := tree.StopAndAddRewriteTime()
	if elapsed < 1 {
		t.Errorf("elapsed should be >= 1 ns, got %d", elapsed)
	}

	total := tree.GetRewriteTime()
	if total < 1 {
		t.Errorf("total rewrite time should be >= 1 ns, got %d", total)
	}

	tree.StartRewriteTime()
	elapsed2 := tree.StopAndAddRewriteTime()
	if elapsed2 < 1 {
		t.Errorf("second elapsed should be >= 1 ns, got %d", elapsed2)
	}

	total2 := tree.GetRewriteTime()
	if total2 < total+1 {
		t.Errorf("rewrite time should have increased; before=%d after=%d", total, total2)
	}
}

// TestQueryProfilerTree_BreakdownNotNil verifies that the returned breakdown
// is usable (non-nil, can retrieve timers).
func TestQueryProfilerTree_BreakdownNotNil(t *testing.T) {
	tree := NewQueryProfilerTree()
	q := &stubQuery{label: "q"}
	bd := tree.GetProfileBreakdown(q)
	if bd == nil {
		t.Fatal("expected non-nil QueryProfilerBreakdown")
	}
	timer := bd.GetTimer(TimingTypeBuildScorer)
	if timer == nil {
		t.Fatal("expected non-nil timer for TimingTypeBuildScorer")
	}
}

// TestQueryProfilerTree_GetTreeReturnsBreakdownData verifies that the result
// tree contains timing data from recorded timers.
func TestQueryProfilerTree_GetTreeReturnsBreakdownData(t *testing.T) {
	tree := NewQueryProfilerTree()
	q := &stubQuery{label: "scored"}
	bd := tree.GetProfileBreakdown(q)

	// Record some time under the build-scorer timer.
	timer := bd.GetTimer(TimingTypeBuildScorer)
	timer.Start()
	timer.Stop()

	tree.PollLast()

	results := tree.GetTree()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	breakdown := results[0].GetTimeBreakdown()
	if len(breakdown) == 0 {
		t.Error("expected non-empty breakdown map")
	}
}
