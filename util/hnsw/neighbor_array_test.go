// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// scoringFuncScorer is a test-only RandomVectorScorer that delegates
// Score to a func. Diversity-related tests use the Updateable variant
// below.
type scoringFuncScorer struct {
	score func(node int) (float32, error)
}

func (s *scoringFuncScorer) Score(node int) (float32, error) {
	return s.score(node)
}

func (s *scoringFuncScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return BulkScoreDefault(s, nodes, scores, numNodes)
}

func (s *scoringFuncScorer) MaxOrd() int                       { panic("unsupported") }
func (s *scoringFuncScorer) OrdToDoc(int) int                  { panic("unsupported") }
func (s *scoringFuncScorer) GetAcceptOrds(util.Bits) util.Bits { panic("unsupported") }

// scoringFuncUpdateable wraps scoringFuncScorer with a settable
// scoring ordinal. Tests that exercise AddAndEnsureDiversity use this.
type scoringFuncUpdateable struct {
	*scoringFuncScorer
	current int
	pair    func(a, b int) (float32, error) // optional: scores by (current,target)
}

func (s *scoringFuncUpdateable) SetScoringOrdinal(node int) error {
	s.current = node
	return nil
}

func (s *scoringFuncUpdateable) Score(node int) (float32, error) {
	if s.pair != nil {
		return s.pair(s.current, node)
	}
	return s.scoringFuncScorer.Score(node)
}

// expectPanic runs fn and reports an error to t if it does not panic
// with a message satisfying matcher. If matcher is nil, any panic
// passes.
func expectPanic(t *testing.T, matcher func(string) bool, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic, got none")
			return
		}
		if matcher == nil {
			return
		}
		msg := fmt.Sprintf("%v", r)
		if !matcher(msg) {
			t.Errorf("panic message %q failed matcher", msg)
		}
	}()
	fn()
}

func assertScoresEqual(t *testing.T, want []float32, na *NeighborArray) {
	t.Helper()
	for i, w := range want {
		got := na.GetScore(i)
		// Treat NaN as equal to NaN for assertion convenience, since
		// Java's assertEquals(float, float, 0.01f) also accepts
		// NaN == NaN under the standard JUnit semantics used in the
		// reference test.
		if isNaN32(w) && isNaN32(got) {
			continue
		}
		diff := float64(got) - float64(w)
		if math.Abs(diff) > 0.01 {
			t.Errorf("scores[%d]: got %v want %v", i, got, w)
		}
	}
	if na.Size() < len(want) {
		t.Errorf("Size %d < expected %d", na.Size(), len(want))
	}
}

func assertNodesEqual(t *testing.T, want []int, na *NeighborArray) {
	t.Helper()
	nodes := na.Nodes()
	for i, w := range want {
		if nodes[i] != w {
			t.Errorf("nodes[%d]: got %d want %d", i, nodes[i], w)
		}
	}
}

// ---------------------------------------------------------------
// Ports of the Java TestNeighborArray test peers.
// ---------------------------------------------------------------

func TestNeighborArray_ScoresDescOrder(t *testing.T) {
	neighbors := NewNeighborArray(10, true)
	neighbors.AddInOrder(0, 1)
	neighbors.AddInOrder(1, 0.8)

	expectPanic(t,
		func(s string) bool { return strings.HasPrefix(s, "Nodes are added in the incorrect order!") },
		func() { neighbors.AddInOrder(2, 0.9) },
	)

	mustInsert(t, neighbors, 3, 0.9)
	assertScoresEqual(t, []float32{1, 0.9, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 3, 1}, neighbors)

	mustInsert(t, neighbors, 4, 1)
	assertScoresEqual(t, []float32{1, 1, 0.9, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 4, 3, 1}, neighbors)

	mustInsert(t, neighbors, 5, 1.1)
	assertScoresEqual(t, []float32{1.1, 1, 1, 0.9, 0.8}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 3, 1}, neighbors)

	mustInsert(t, neighbors, 6, 0.8)
	assertScoresEqual(t, []float32{1.1, 1, 1, 0.9, 0.8, 0.8}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 3, 1, 6}, neighbors)

	mustInsert(t, neighbors, 7, 0.8)
	assertScoresEqual(t, []float32{1.1, 1, 1, 0.9, 0.8, 0.8, 0.8}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 3, 1, 6, 7}, neighbors)

	neighbors.RemoveIndex(2)
	assertScoresEqual(t, []float32{1.1, 1, 0.9, 0.8, 0.8, 0.8}, neighbors)
	assertNodesEqual(t, []int{5, 0, 3, 1, 6, 7}, neighbors)

	neighbors.RemoveIndex(0)
	assertScoresEqual(t, []float32{1, 0.9, 0.8, 0.8, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 3, 1, 6, 7}, neighbors)

	neighbors.RemoveIndex(4)
	assertScoresEqual(t, []float32{1, 0.9, 0.8, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 3, 1, 6}, neighbors)

	neighbors.RemoveLast()
	assertScoresEqual(t, []float32{1, 0.9, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 3, 1}, neighbors)

	mustInsert(t, neighbors, 8, 0.9)
	assertScoresEqual(t, []float32{1, 0.9, 0.9, 0.8}, neighbors)
	assertNodesEqual(t, []int{0, 3, 8, 1}, neighbors)
}

func TestNeighborArray_ScoresAscOrder(t *testing.T) {
	neighbors := NewNeighborArray(10, false)
	neighbors.AddInOrder(0, 0.1)
	neighbors.AddInOrder(1, 0.3)

	expectPanic(t,
		func(s string) bool { return strings.HasPrefix(s, "Nodes are added in the incorrect order!") },
		func() { neighbors.AddInOrder(2, 0.15) },
	)

	mustInsert(t, neighbors, 3, 0.3)
	assertScoresEqual(t, []float32{0.1, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{0, 1, 3}, neighbors)

	mustInsert(t, neighbors, 4, 0.2)
	assertScoresEqual(t, []float32{0.1, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{0, 4, 1, 3}, neighbors)

	mustInsert(t, neighbors, 5, 0.05)
	assertScoresEqual(t, []float32{0.05, 0.1, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 1, 3}, neighbors)

	mustInsert(t, neighbors, 6, 0.2)
	assertScoresEqual(t, []float32{0.05, 0.1, 0.2, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 6, 1, 3}, neighbors)

	mustInsert(t, neighbors, 7, 0.2)
	assertScoresEqual(t, []float32{0.05, 0.1, 0.2, 0.2, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{5, 0, 4, 6, 7, 1, 3}, neighbors)

	neighbors.RemoveIndex(2)
	assertScoresEqual(t, []float32{0.05, 0.1, 0.2, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{5, 0, 6, 7, 1, 3}, neighbors)

	neighbors.RemoveIndex(0)
	assertScoresEqual(t, []float32{0.1, 0.2, 0.2, 0.3, 0.3}, neighbors)
	assertNodesEqual(t, []int{0, 6, 7, 1, 3}, neighbors)

	neighbors.RemoveIndex(4)
	assertScoresEqual(t, []float32{0.1, 0.2, 0.2, 0.3}, neighbors)
	assertNodesEqual(t, []int{0, 6, 7, 1}, neighbors)

	neighbors.RemoveLast()
	assertScoresEqual(t, []float32{0.1, 0.2, 0.2}, neighbors)
	assertNodesEqual(t, []int{0, 6, 7}, neighbors)

	mustInsert(t, neighbors, 8, 0.01)
	assertScoresEqual(t, []float32{0.01, 0.1, 0.2, 0.2}, neighbors)
	assertNodesEqual(t, []int{8, 0, 6, 7}, neighbors)
}

func TestNeighborArray_SortAsc(t *testing.T) {
	neighbors := NewNeighborArray(10, false)
	neighbors.AddOutOfOrder(1, 2)
	// We disallow calling AddInOrder after AddOutOfOrder even if the
	// pair is in fact in order.
	expectPanic(t, nil, func() { neighbors.AddInOrder(1, 2) })
	neighbors.AddOutOfOrder(2, 3)
	neighbors.AddOutOfOrder(5, 6)
	neighbors.AddOutOfOrder(3, 4)
	neighbors.AddOutOfOrder(7, 8)
	neighbors.AddOutOfOrder(6, 7)
	neighbors.AddOutOfOrder(4, 5)
	unchecked, err := neighbors.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{0, 1, 2, 3, 4, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{0, 1, 2, 3, 4, 5, 6})
	}
	assertNodesEqual(t, []int{1, 2, 3, 4, 5, 6, 7}, neighbors)
	assertScoresEqual(t, []float32{2, 3, 4, 5, 6, 7, 8}, neighbors)

	neighbors2 := NewNeighborArray(10, false)
	neighbors2.AddInOrder(0, 1)
	neighbors2.AddInOrder(1, 2)
	neighbors2.AddInOrder(4, 5)
	neighbors2.AddOutOfOrder(2, 3)
	neighbors2.AddOutOfOrder(6, 7)
	neighbors2.AddOutOfOrder(5, 6)
	neighbors2.AddOutOfOrder(3, 4)
	unchecked, err = neighbors2.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{2, 3, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{2, 3, 5, 6})
	}
	assertNodesEqual(t, []int{0, 1, 2, 3, 4, 5, 6}, neighbors2)
	assertScoresEqual(t, []float32{1, 2, 3, 4, 5, 6, 7}, neighbors2)
}

func TestNeighborArray_SortDesc(t *testing.T) {
	neighbors := NewNeighborArray(10, true)
	neighbors.AddOutOfOrder(1, 7)
	expectPanic(t, nil, func() { neighbors.AddInOrder(1, 2) })
	neighbors.AddOutOfOrder(2, 6)
	neighbors.AddOutOfOrder(5, 3)
	neighbors.AddOutOfOrder(3, 5)
	neighbors.AddOutOfOrder(7, 1)
	neighbors.AddOutOfOrder(6, 2)
	neighbors.AddOutOfOrder(4, 4)
	unchecked, err := neighbors.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{0, 1, 2, 3, 4, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{0, 1, 2, 3, 4, 5, 6})
	}
	assertNodesEqual(t, []int{1, 2, 3, 4, 5, 6, 7}, neighbors)
	assertScoresEqual(t, []float32{7, 6, 5, 4, 3, 2, 1}, neighbors)

	neighbors2 := NewNeighborArray(10, true)
	neighbors2.AddInOrder(1, 7)
	neighbors2.AddInOrder(2, 6)
	neighbors2.AddInOrder(5, 3)
	neighbors2.AddOutOfOrder(3, 5)
	neighbors2.AddOutOfOrder(7, 1)
	neighbors2.AddOutOfOrder(6, 2)
	neighbors2.AddOutOfOrder(4, 4)
	unchecked, err = neighbors2.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{2, 3, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{2, 3, 5, 6})
	}
	assertNodesEqual(t, []int{1, 2, 3, 4, 5, 6, 7}, neighbors2)
	assertScoresEqual(t, []float32{7, 6, 5, 4, 3, 2, 1}, neighbors2)
}

func TestNeighborArray_AddWithScoringFunction(t *testing.T) {
	neighbors := NewNeighborArray(10, true)
	neighbors.AddOutOfOrder(1, nan())
	expectPanic(t, nil, func() { neighbors.AddInOrder(1, 2) })
	neighbors.AddOutOfOrder(2, nan())
	neighbors.AddOutOfOrder(5, nan())
	neighbors.AddOutOfOrder(3, nan())
	neighbors.AddOutOfOrder(7, nan())
	neighbors.AddOutOfOrder(6, nan())
	neighbors.AddOutOfOrder(4, nan())
	scorer := &scoringFuncScorer{
		score: func(node int) (float32, error) { return float32(7 - node + 1), nil },
	}
	unchecked, err := neighbors.Sort(scorer)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{0, 1, 2, 3, 4, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{0, 1, 2, 3, 4, 5, 6})
	}
	assertNodesEqual(t, []int{1, 2, 3, 4, 5, 6, 7}, neighbors)
	assertScoresEqual(t, []float32{7, 6, 5, 4, 3, 2, 1}, neighbors)
}

func TestNeighborArray_AddWithScoringFunctionLargeOrd(t *testing.T) {
	neighbors := NewNeighborArray(10, true)
	neighbors.AddOutOfOrder(11, nan())
	expectPanic(t, nil, func() { neighbors.AddInOrder(1, 2) })
	neighbors.AddOutOfOrder(12, nan())
	neighbors.AddOutOfOrder(15, nan())
	neighbors.AddOutOfOrder(13, nan())
	neighbors.AddOutOfOrder(17, nan())
	neighbors.AddOutOfOrder(16, nan())
	neighbors.AddOutOfOrder(14, nan())
	scorer := &scoringFuncScorer{
		score: func(node int) (float32, error) { return float32(7 - node + 11), nil },
	}
	unchecked, err := neighbors.Sort(scorer)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if !reflect.DeepEqual(unchecked, []int{0, 1, 2, 3, 4, 5, 6}) {
		t.Errorf("unchecked: got %v want %v", unchecked, []int{0, 1, 2, 3, 4, 5, 6})
	}
	assertNodesEqual(t, []int{11, 12, 13, 14, 15, 16, 17}, neighbors)
	assertScoresEqual(t, []float32{7, 6, 5, 4, 3, 2, 1}, neighbors)
}

func TestNeighborArray_MaxSizeAddInOrder(t *testing.T) {
	neighbors := NewNeighborArray(3, true)
	neighbors.AddInOrder(0, 1.0)
	neighbors.AddInOrder(1, 0.8)
	neighbors.AddInOrder(2, 0.6)
	expectPanic(t,
		func(s string) bool { return strings.Contains(s, "No growth is allowed") },
		func() { neighbors.AddInOrder(3, 0.4) },
	)
	assertScoresEqual(t, []float32{1.0, 0.8, 0.6}, neighbors)
	assertNodesEqual(t, []int{0, 1, 2}, neighbors)
}

func TestNeighborArray_MaxSizeAddOutOfOrder(t *testing.T) {
	neighbors := NewNeighborArray(3, true)
	neighbors.AddOutOfOrder(0, 0.8)
	neighbors.AddOutOfOrder(1, 1.0)
	neighbors.AddOutOfOrder(2, 0.6)
	expectPanic(t,
		func(s string) bool { return strings.Contains(s, "No growth is allowed") },
		func() { neighbors.AddOutOfOrder(3, 0.4) },
	)
	if _, err := neighbors.Sort(nil); err != nil {
		t.Fatalf("Sort: %v", err)
	}
	assertScoresEqual(t, []float32{1.0, 0.8, 0.6}, neighbors)
	assertNodesEqual(t, []int{1, 0, 2}, neighbors)
}

func TestNeighborArray_MaxSizeInsertSorted(t *testing.T) {
	neighbors := NewNeighborArray(3, true)
	mustInsert(t, neighbors, 0, 1.0)
	mustInsert(t, neighbors, 1, 0.8)
	mustInsert(t, neighbors, 2, 0.6)
	expectPanic(t,
		func(s string) bool { return strings.Contains(s, "No growth is allowed") },
		func() { _ = neighbors.InsertSorted(3, 0.4) },
	)
	assertScoresEqual(t, []float32{1.0, 0.8, 0.6}, neighbors)
	assertNodesEqual(t, []int{0, 1, 2}, neighbors)
}

func TestNeighborArray_MaxSizeMixedOperations(t *testing.T) {
	neighbors := NewNeighborArray(3, true)
	neighbors.AddInOrder(0, 1.0)
	neighbors.AddOutOfOrder(1, 0.8)
	expectPanic(t, nil, func() { neighbors.AddInOrder(2, 0.6) })
	neighbors.AddOutOfOrder(2, 0.6)
	expectPanic(t,
		func(s string) bool { return strings.Contains(s, "No growth is allowed") },
		func() { neighbors.AddOutOfOrder(3, 0.4) },
	)
	if _, err := neighbors.Sort(nil); err != nil {
		t.Fatalf("Sort: %v", err)
	}
	assertScoresEqual(t, []float32{1.0, 0.8, 0.6}, neighbors)
	assertNodesEqual(t, []int{0, 1, 2}, neighbors)
}

func TestNeighborArray_BoundaryValues(t *testing.T) {
	neighbors := NewNeighborArray(3, true)
	if neighbors.Size() != 0 {
		t.Fatalf("Size: got %d want 0", neighbors.Size())
	}

	neighbors.AddInOrder(0, math.MaxFloat32)
	assertScoresEqual(t, []float32{math.MaxFloat32}, neighbors)
	assertNodesEqual(t, []int{0}, neighbors)

	neighbors.Clear()
	neighbors.AddOutOfOrder(0, nan())
	neighbors.AddOutOfOrder(1, nan())
	assertScoresEqual(t, []float32{nan(), nan()}, neighbors)
	assertNodesEqual(t, []int{0, 1}, neighbors)
}

func TestNeighborArray_SortOrder(t *testing.T) {
	asc := NewNeighborArray(5, false)
	asc.AddInOrder(0, 0.1)
	asc.AddInOrder(1, 0.2)
	asc.AddInOrder(2, 0.3)
	assertScoresEqual(t, []float32{0.1, 0.2, 0.3}, asc)

	desc := NewNeighborArray(5, true)
	desc.AddInOrder(0, 0.3)
	desc.AddInOrder(1, 0.2)
	desc.AddInOrder(2, 0.1)
	assertScoresEqual(t, []float32{0.3, 0.2, 0.1}, desc)

	eq := NewNeighborArray(5, true)
	eq.AddInOrder(0, 0.5)
	eq.AddInOrder(1, 0.5)
	eq.AddInOrder(2, 0.5)
	assertScoresEqual(t, []float32{0.5, 0.5, 0.5}, eq)
}

func TestNeighborArray_RemoveOperations(t *testing.T) {
	neighbors := NewNeighborArray(5, true)
	neighbors.AddInOrder(0, 1.0)
	neighbors.AddInOrder(1, 0.8)
	neighbors.AddInOrder(2, 0.6)
	neighbors.AddInOrder(3, 0.4)

	neighbors.RemoveLast()
	assertScoresEqual(t, []float32{1.0, 0.8, 0.6}, neighbors)
	assertNodesEqual(t, []int{0, 1, 2}, neighbors)

	neighbors.RemoveIndex(1)
	assertScoresEqual(t, []float32{1.0, 0.6}, neighbors)
	assertNodesEqual(t, []int{0, 2}, neighbors)

	neighbors.RemoveIndex(0)
	assertScoresEqual(t, []float32{0.6}, neighbors)
	assertNodesEqual(t, []int{2}, neighbors)

	neighbors.RemoveLast()
	if neighbors.Size() != 0 {
		t.Fatalf("Size: got %d want 0", neighbors.Size())
	}
}

func TestNeighborArray_ClearOperation(t *testing.T) {
	neighbors := NewNeighborArray(5, true)
	neighbors.AddInOrder(0, 1.0)
	neighbors.AddInOrder(1, 0.8)
	neighbors.AddInOrder(2, 0.6)
	neighbors.Clear()
	if neighbors.Size() != 0 {
		t.Fatalf("Size after Clear: got %d want 0", neighbors.Size())
	}
	neighbors.AddInOrder(3, 1.0)
	assertScoresEqual(t, []float32{1.0}, neighbors)
	assertNodesEqual(t, []int{3}, neighbors)
}

func TestNeighborArray_ComplexOperations(t *testing.T) {
	neighbors := NewNeighborArray(5, true)
	neighbors.AddInOrder(0, 1.0)
	neighbors.AddInOrder(1, 0.8)
	neighbors.AddOutOfOrder(2, 0.9)
	neighbors.AddOutOfOrder(3, 0.7)
	if _, err := neighbors.Sort(nil); err != nil {
		t.Fatalf("Sort: %v", err)
	}
	assertScoresEqual(t, []float32{1.0, 0.9, 0.8, 0.7}, neighbors)
	assertNodesEqual(t, []int{0, 2, 1, 3}, neighbors)

	neighbors.RemoveIndex(1)
	neighbors.RemoveLast()
	mustInsert(t, neighbors, 4, 0.85)
	mustInsert(t, neighbors, 5, 0.75)
	assertScoresEqual(t, []float32{1.0, 0.85, 0.8, 0.75}, neighbors)
	assertNodesEqual(t, []int{0, 4, 1, 5}, neighbors)
}

func TestNeighborArray_ScorerOperations(t *testing.T) {
	neighbors := NewNeighborArray(5, true)
	scorer := &scoringFuncScorer{
		score: func(node int) (float32, error) {
			switch node {
			case 0:
				return 1.0, nil
			case 1:
				return 0.8, nil
			case 2:
				return 0.9, nil
			case 3:
				return 0.7, nil
			default:
				return 0.0, nil
			}
		},
	}
	neighbors.AddOutOfOrder(0, nan())
	neighbors.AddOutOfOrder(1, nan())
	neighbors.AddOutOfOrder(2, nan())
	neighbors.AddOutOfOrder(3, nan())
	if _, err := neighbors.Sort(scorer); err != nil {
		t.Fatalf("Sort: %v", err)
	}
	assertScoresEqual(t, []float32{1.0, 0.9, 0.8, 0.7}, neighbors)
	assertNodesEqual(t, []int{0, 2, 1, 3}, neighbors)
}

// ---------------------------------------------------------------
// Go-native coverage: surface, accessors, diversity path.
// ---------------------------------------------------------------

func TestNeighborArray_MaxSizeAndOrderAccessors(t *testing.T) {
	na := NewNeighborArray(7, true)
	if na.MaxSize() != 7 {
		t.Errorf("MaxSize: got %d want 7", na.MaxSize())
	}
	if !na.ScoresDescOrder() {
		t.Errorf("ScoresDescOrder: got false want true")
	}
	na2 := NewNeighborArray(7, false)
	if na2.ScoresDescOrder() {
		t.Errorf("ScoresDescOrder: got true want false")
	}
}

func TestNeighborArray_StringFormat(t *testing.T) {
	na := NewNeighborArray(5, true)
	na.AddInOrder(0, 1.0)
	na.AddInOrder(1, 0.5)
	got := fmt.Sprintf("%s", na)
	if got != "NeighborArray[2]" {
		t.Errorf("String: got %q want %q", got, "NeighborArray[2]")
	}
}

func TestNeighborArray_SortNoopWhenSorted(t *testing.T) {
	na := NewNeighborArray(5, true)
	na.AddInOrder(0, 1.0)
	na.AddInOrder(1, 0.5)
	got, err := na.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if got != nil {
		t.Errorf("Sort already-sorted: got %v want nil", got)
	}
}

func TestNeighborArray_SortNaNWithoutScorer(t *testing.T) {
	na := NewNeighborArray(5, true)
	na.AddOutOfOrder(0, nan())
	if _, err := na.Sort(nil); err == nil {
		t.Errorf("Sort with NaN and nil scorer: expected error, got nil")
	}
}

func TestNeighborArray_AddAndEnsureDiversity_BelowCap(t *testing.T) {
	na := NewNeighborArray(5, true)
	upd := &scoringFuncUpdateable{scoringFuncScorer: &scoringFuncScorer{
		score: func(node int) (float32, error) { return 0, nil },
	}}
	na.AddInOrder(10, 1.0)
	na.AddInOrder(20, 0.9)
	if err := na.AddAndEnsureDiversity(30, 0.8, 0, upd); err != nil {
		t.Fatalf("AddAndEnsureDiversity: %v", err)
	}
	if na.Size() != 3 {
		t.Errorf("Size: got %d want 3 (no eviction below max)", na.Size())
	}
}

func TestNeighborArray_AddAndEnsureDiversity_FullEvictsLast(t *testing.T) {
	// All neighbors are perfectly diverse from each other (pair score
	// always 0); the worst-non-diverse fallback path returns size-1.
	na := NewNeighborArray(3, true)
	na.AddInOrder(10, 0.9)
	na.AddInOrder(20, 0.8)
	upd := &scoringFuncUpdateable{
		scoringFuncScorer: &scoringFuncScorer{score: func(int) (float32, error) { return 0, nil }},
		pair:              func(int, int) (float32, error) { return 0, nil },
	}
	if err := na.AddAndEnsureDiversity(30, 0.7, 99, upd); err != nil {
		t.Fatalf("AddAndEnsureDiversity: %v", err)
	}
	// After eviction the array size returns to maxSize-1 = 2 because
	// the new entry triggered the eviction. Nodes left: the two
	// highest-scoring of the original three are retained.
	if na.Size() != 2 {
		t.Errorf("Size: got %d want 2", na.Size())
	}
	assertScoresEqual(t, []float32{0.9, 0.8}, na)
	assertNodesEqual(t, []int{10, 20}, na)
}

func TestNeighborArray_AddAndEnsureDiversity_EvictsNonDiverse(t *testing.T) {
	// Two existing neighbors (10 -> 0.9, 20 -> 0.8); a third (30 ->
	// 0.7) gets inserted. We declare 30 to be too similar to 10
	// (pair score >= 0.7), so the unchecked candidate 30 itself is
	// the worst non-diverse and gets evicted, returning to {10, 20}.
	na := NewNeighborArray(3, true)
	na.AddInOrder(10, 0.9)
	na.AddInOrder(20, 0.8)
	upd := &scoringFuncUpdateable{
		scoringFuncScorer: &scoringFuncScorer{score: func(int) (float32, error) { return 0, nil }},
		pair: func(setOrd, target int) (float32, error) {
			// setOrd is the SetScoringOrdinal value, target is the
			// node being scored against. Make every pair pass the
			// "neighbor too similar" threshold so the unchecked
			// candidate is the first non-diverse.
			return 1.0, nil
		},
	}
	if err := na.AddAndEnsureDiversity(30, 0.7, 99, upd); err != nil {
		t.Fatalf("AddAndEnsureDiversity: %v", err)
	}
	if na.Size() != 2 {
		t.Errorf("Size: got %d want 2", na.Size())
	}
	assertScoresEqual(t, []float32{0.9, 0.8}, na)
	assertNodesEqual(t, []int{10, 20}, na)
}

// TestNeighborArray_FloatCompare_NaNOrdering exercises the binary
// search path with NaN entries, which under Java's Float.compare are
// greater than +Inf and equal to themselves. This guarantees the
// ascending insertion-point helper handles NaN deterministically.
func TestNeighborArray_FloatCompare_NaNOrdering(t *testing.T) {
	if c := cmpFloat32(nan(), nan()); c != 0 {
		t.Errorf("cmpFloat32(NaN, NaN): got %d want 0", c)
	}
	if c := cmpFloat32(nan(), float32(math.Inf(1))); c != 1 {
		t.Errorf("cmpFloat32(NaN, +Inf): got %d want 1", c)
	}
	if c := cmpFloat32(float32(math.Inf(1)), nan()); c != -1 {
		t.Errorf("cmpFloat32(+Inf, NaN): got %d want -1", c)
	}
	if c := cmpFloat32(1.0, 2.0); c != -1 {
		t.Errorf("cmpFloat32(1, 2): got %d want -1", c)
	}
	if c := cmpFloat32(2.0, 1.0); c != 1 {
		t.Errorf("cmpFloat32(2, 1): got %d want 1", c)
	}
}

func TestNeighborArray_RemoveLast_SortedNodeSizeClamps(t *testing.T) {
	// After AddOutOfOrder followed by RemoveLast, sortedNodeSize must
	// clamp to size; otherwise a subsequent Sort would over-shift.
	na := NewNeighborArray(5, true)
	na.AddInOrder(0, 1.0)
	na.AddInOrder(1, 0.8)
	na.AddOutOfOrder(2, 0.9)
	na.RemoveLast()
	got, err := na.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if got != nil {
		t.Errorf("Sort after RemoveLast: got %v want nil (already sorted)", got)
	}
	if na.Size() != 2 {
		t.Errorf("Size: got %d want 2", na.Size())
	}
}

func TestNeighborArray_InsertSortedUpdatesSortedSize(t *testing.T) {
	// Repeated InsertSorted on an ascending-order array must keep
	// the result fully sorted (sortedNodeSize == size) so that a
	// subsequent Sort returns nil.
	na := NewNeighborArray(8, false)
	scores := []float32{0.5, 0.1, 0.7, 0.2, 0.9, 0.3, 0.6}
	nodes := []int{0, 1, 2, 3, 4, 5, 6}
	for i, s := range scores {
		mustInsert(t, na, nodes[i], s)
	}
	got, err := na.Sort(nil)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if got != nil {
		t.Errorf("Sort after InsertSorted run: got %v want nil", got)
	}
	want := []float32{0.1, 0.2, 0.3, 0.5, 0.6, 0.7, 0.9}
	assertScoresEqual(t, want, na)
}

// ---------------------------------------------------------------
// Helpers used by the table tests above.
// ---------------------------------------------------------------

func mustInsert(t *testing.T, na *NeighborArray, node int, score float32) {
	t.Helper()
	if err := na.InsertSorted(node, score); err != nil {
		t.Fatalf("InsertSorted(%d,%v): %v", node, score, err)
	}
}

func nan() float32 { return float32(math.NaN()) }
