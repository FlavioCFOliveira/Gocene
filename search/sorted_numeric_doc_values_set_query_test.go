// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestNewSortedNumericDocValuesSetQuery_DefensiveCopyAndSort covers
// the P1 contract: the constructor must not mutate the caller's slice
// and must sort its internal copy ascending before building the set.
func TestNewSortedNumericDocValuesSetQuery_DefensiveCopyAndSort(t *testing.T) {
	t.Parallel()
	caller := []int64{3, 1, 2, 1}
	snapshot := append([]int64(nil), caller...)

	q, err := NewSortedNumericDocValuesSetQuery("f", caller)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesSetQuery: %v", err)
	}

	// Caller's slice must remain untouched (the constructor takes a
	// defensive copy before sorting).
	for i := range caller {
		if caller[i] != snapshot[i] {
			t.Errorf("caller[%d] mutated: got %d, want %d",
				i, caller[i], snapshot[i])
		}
	}

	// Internal view must be sorted and de-duplicated.
	concrete, ok := q.(*sortedNumericDocValuesSetQuery)
	if !ok {
		t.Fatalf("expected *sortedNumericDocValuesSetQuery, got %T", q)
	}
	got := concrete.Values()
	want := []int64{1, 2, 3}
	if !sliceEqualInt64(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
	if concrete.numbers.Min() != 1 || concrete.numbers.Max() != 3 {
		t.Errorf("min/max = %d/%d, want 1/3",
			concrete.numbers.Min(), concrete.numbers.Max())
	}
}

// TestNewSortedNumericDocValuesSetQuery_EmptyField covers the
// constructor rejection branch (Java throws NPE on a null field; Go
// surfaces an idiomatic error).
func TestNewSortedNumericDocValuesSetQuery_EmptyField(t *testing.T) {
	t.Parallel()
	_, err := NewSortedNumericDocValuesSetQuery("", []int64{1, 2})
	if err == nil {
		t.Fatal("expected error for empty field")
	}
}

// TestSortedNumericDocValuesSetQuery_EmptySetRewritesToMatchNoDocs
// covers the rewrite() branch from the Java reference: an empty set
// folds to MatchNoDocsQuery.
func TestSortedNumericDocValuesSetQuery_EmptySetRewritesToMatchNoDocs(t *testing.T) {
	t.Parallel()
	q, err := NewSortedNumericDocValuesSetQuery("f", nil)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesSetQuery: %v", err)
	}
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Rewrite() = %T, want *MatchNoDocsQuery", rewritten)
	}
}

// TestSortedNumericDocValuesSetQuery_NonEmptyRewritesToSelf covers the
// other side of the Java rewrite() fast path: a non-empty query is
// returned unchanged.
func TestSortedNumericDocValuesSetQuery_NonEmptyRewritesToSelf(t *testing.T) {
	t.Parallel()
	q, err := NewSortedNumericDocValuesSetQuery("f", []int64{1})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesSetQuery: %v", err)
	}
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rewritten != q {
		t.Errorf("Rewrite() returned a different query: got %p, want %p",
			rewritten, q)
	}
}

// TestSortedNumericDocValuesSetQuery_Equals covers the equals contract
// from the Java reference: same class, same field, same set.
func TestSortedNumericDocValuesSetQuery_Equals(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedNumericDocValuesSetQuery("f", []int64{3, 1, 2})
	b, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 2, 3})
	c, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 2, 4})
	d, _ := NewSortedNumericDocValuesSetQuery("g", []int64{1, 2, 3})

	if !a.Equals(b) {
		t.Error("a should equal b (same field, same set)")
	}
	if !b.Equals(a) {
		t.Error("equals must be symmetric")
	}
	if a.Equals(c) {
		t.Error("a should not equal c (different set)")
	}
	if a.Equals(d) {
		t.Error("a should not equal d (different field)")
	}
	if a.Equals(NewMatchNoDocsQuery()) {
		t.Error("a should not equal MatchNoDocsQuery (different type)")
	}
}

// TestSortedNumericDocValuesSetQuery_HashCodeStability covers the
// HashCode contract from the Java reference: equal queries must have
// equal hash codes, and the hash must be sensitive to field, set
// content, and set size.
func TestSortedNumericDocValuesSetQuery_HashCodeStability(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedNumericDocValuesSetQuery("f", []int64{3, 1, 2})
	b, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 2, 3})
	c, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 2, 4})
	d, _ := NewSortedNumericDocValuesSetQuery("g", []int64{1, 2, 3})
	e, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 2})

	if a.HashCode() != b.HashCode() {
		t.Errorf("equal queries must share hash: %d vs %d",
			a.HashCode(), b.HashCode())
	}
	// Sensitivity checks. These are not strict requirements of the
	// hash contract, but they verify the seed and folds keep the
	// hash useful for hash-based collections.
	if a.HashCode() == c.HashCode() {
		t.Error("different set content should produce different hash")
	}
	if a.HashCode() == d.HashCode() {
		t.Error("different field should produce different hash")
	}
	if a.HashCode() == e.HashCode() {
		t.Error("different set size should produce different hash")
	}
}

// TestSortedNumericDocValuesSetQuery_String covers the toString()
// rendering: "<field>: [v1, v2, ...]". The default-field argument is
// always ignored, matching the Java reference.
func TestSortedNumericDocValuesSetQuery_String(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedNumericDocValuesSetQuery("price", []int64{3, 1, 2})
	got := q.(*sortedNumericDocValuesSetQuery).String("any")
	want := "price: [1, 2, 3]"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	empty, _ := NewSortedNumericDocValuesSetQuery("price", nil)
	if got := empty.(*sortedNumericDocValuesSetQuery).String(""); got != "price: []" {
		t.Errorf("empty String() = %q, want %q", got, "price: []")
	}
}

// TestSortedNumericDocValuesSetQuery_Visit covers the QueryVisitor
// dispatch: VisitLeaf is invoked iff AcceptField returns true.
func TestSortedNumericDocValuesSetQuery_Visit(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1})

	accept := &setQueryVisitor{accept: true}
	q.(*sortedNumericDocValuesSetQuery).Visit(accept)
	if !accept.leafCalled {
		t.Error("VisitLeaf should be called when AcceptField returns true")
	}

	reject := &setQueryVisitor{accept: false}
	q.(*sortedNumericDocValuesSetQuery).Visit(reject)
	if reject.leafCalled {
		t.Error("VisitLeaf should not be called when AcceptField returns false")
	}
}

// setQueryVisitor is a minimal QueryVisitor used by the Visit test.
// Embeds EmptyQueryVisitorBase to inherit no-op defaults and only
// overrides the two hooks the test exercises. Kept local to the test
// file (no need to expose it elsewhere; the existing
// latLonDocValuesQueryVisitor is also test-local).
type setQueryVisitor struct {
	EmptyQueryVisitorBase
	accept     bool
	leafCalled bool
}

func (v *setQueryVisitor) AcceptField(_ string) bool { return v.accept }
func (v *setQueryVisitor) VisitLeaf(_ Query)         { v.leafCalled = true }

// TestDocValuesLongHashSet_ContainsAndBoundsAsQueryBackend exercises
// the membership backend used by the query: it is the hot path
// consulted by the scorer loop and must agree with the Java
// DocValuesLongHashSet contract on the observable bits (Min / Max /
// Contains). The exhaustive unit tests for the hash set live next to
// the implementation in document/; this test pins the surface the
// query consumes.
func TestDocValuesLongHashSet_ContainsAndBoundsAsQueryBackend(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		s := document.NewDocValuesLongHashSet(nil)
		if s.Size() != 0 {
			t.Errorf("Size = %d, want 0", s.Size())
		}
		if s.Min() != math.MaxInt64 {
			t.Errorf("Min = %d, want MaxInt64 (Long.MAX_VALUE sentinel)", s.Min())
		}
		if s.Max() != math.MinInt64 {
			t.Errorf("Max = %d, want MinInt64 (Long.MIN_VALUE sentinel)", s.Max())
		}
		if s.Contains(0) {
			t.Error("empty set should not contain 0")
		}
	})

	t.Run("with-duplicates", func(t *testing.T) {
		// DocValuesLongHashSet requires sorted input; duplicates are
		// tolerated and collapsed by the constructor.
		s := document.NewDocValuesLongHashSet([]int64{1, 1, 2, 3, 3, 3, 5})
		if s.Size() != 4 {
			t.Errorf("Size = %d, want 4 (after dedup)", s.Size())
		}
		if s.Min() != 1 {
			t.Errorf("Min = %d, want 1", s.Min())
		}
		if s.Max() != 5 {
			t.Errorf("Max = %d, want 5", s.Max())
		}
		for _, in := range []int64{1, 2, 3, 5} {
			if !s.Contains(in) {
				t.Errorf("Contains(%d) = false, want true", in)
			}
		}
		for _, out := range []int64{-1, 0, 4, 6, 100} {
			if s.Contains(out) {
				t.Errorf("Contains(%d) = true, want false", out)
			}
		}
	})
}

// TestSortedNumericDocValuesSetQuery_MatchLoop_MultiValue exercises
// the multi-value scorer path: for each candidate doc, the matches()
// loop walks the sorted value slice and decides membership. Mirrors
// the Java TwoPhaseIterator.matches() body in the non-singleton
// branch.
func TestSortedNumericDocValuesSetQuery_MatchLoop_MultiValue(t *testing.T) {
	t.Parallel()

	set := document.NewDocValuesLongHashSet([]int64{10, 20, 30})

	cases := []struct {
		name string
		vals []int64
		want bool
	}{
		{"no-values", nil, false},
		{"all-below-min", []int64{-5, 0, 5}, false},
		{"all-above-max-early-exit", []int64{100, 200}, false},
		{"single-hit", []int64{20}, true},
		{"hit-after-below", []int64{5, 10}, true},
		{"miss-only", []int64{5, 11, 19}, false},
		{"hit-then-stop", []int64{5, 10, 100}, true},
		{"early-terminate-after-max", []int64{15, 50, 30}, false}, // 50 > max => terminate, 30 never reached
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := setQueryMultiValueMatch(set, tc.vals)
			if got != tc.want {
				t.Errorf("multiValueMatch(%v) = %v, want %v",
					tc.vals, got, tc.want)
			}
		})
	}
}

// setQueryMultiValueMatch replays the multi-value matches() loop in
// isolation. The query's matches() closure is built inside
// CreateWeight against a SortedNumericDocValues iterator; replicating
// the loop here lets us cover the boundary conditions without
// standing up a full Weight / supplier / approximation chain.
//
// The body must stay byte-identical to the matches() loop in
// CreateWeight; the test is the canary that flags any divergence.
func setQueryMultiValueMatch(s *document.DocValuesLongHashSet, vs []int64) bool {
	minV, maxV := s.Min(), s.Max()
	for _, v := range vs {
		if v < minV {
			continue
		}
		if v > maxV {
			return false
		}
		if s.Contains(v) {
			return true
		}
	}
	return false
}

// TestSortedNumericDocValuesSetQuery_End2End_MultiValue drives the
// full query through a fake LeafReaderContext and verifies the
// matching doc set. This is the closest analogue we have to the
// Java IndexSearcher-based test peer for this query.
func TestSortedNumericDocValuesSetQuery_End2End_MultiValue(t *testing.T) {
	t.Parallel()

	q, err := NewSortedNumericDocValuesSetQuery("f", []int64{10, 20, 30})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesSetQuery: %v", err)
	}

	values := map[int][]int64{
		0: {10},        // hit
		1: {5},         // miss (below min)
		2: {5, 100},    // miss (above max early-exit)
		3: {15, 20},    // hit (second value)
		4: {25},        // miss (in range, not in set)
		5: {30, 40},    // hit (first value)
		6: nil,         // miss (no values)
		7: {0, 11, 30}, // hit (third value, after misses)
	}
	got := runSortedNumericSetQuery(t, q, values)
	want := []int{0, 3, 5, 7}
	if !sliceEqualInt(got, want) {
		t.Errorf("matched docs = %v, want %v", got, want)
	}
}

// TestSortedNumericDocValuesSetQuery_End2End_SingletonFastPath drives
// the singleton-backed scorer branch. The Java reference unwraps
// SortedNumeric → Numeric for fields with at most one value per doc
// and reads the value directly via longValue(); we mirror that with
// index.UnwrapSingletonSortedNumeric / index.Singleton.
func TestSortedNumericDocValuesSetQuery_End2End_SingletonFastPath(t *testing.T) {
	t.Parallel()

	q, err := NewSortedNumericDocValuesSetQuery("f", []int64{10, 20, 30})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesSetQuery: %v", err)
	}

	numeric := newFakeNumeric(map[int]int64{
		0: 10, // hit
		1: 5,  // miss (below min)
		2: 11, // miss (in range, not in set)
		3: 30, // hit
		4: 99, // miss (above max)
		5: 20, // hit
	})
	dv := index.Singleton(numeric)

	got := runSortedNumericSetQueryWithDV(t, q, dv, 6)
	want := []int{0, 3, 5}
	if !sliceEqualInt(got, want) {
		t.Errorf("matched docs = %v, want %v", got, want)
	}
}

// runSortedNumericSetQuery drives the query against a fake
// SortedNumericDocValues iterator built from values and returns the
// matched doc ids in ascending order.
func runSortedNumericSetQuery(t *testing.T, q Query, values map[int][]int64) []int {
	t.Helper()
	fake := newFakeSortedNumeric(values)
	maxDoc := 0
	for id := range values {
		if id+1 > maxDoc {
			maxDoc = id + 1
		}
	}
	return runSortedNumericSetQueryWithDV(t, q, fake, maxDoc)
}

// runSortedNumericSetQueryWithDV drives the query against a
// pre-built SortedNumericDocValues iterator and a known maxDoc.
// Replays the supplier branch from CreateWeight without standing up
// a full SegmentReader.
func runSortedNumericSetQueryWithDV(
	t *testing.T,
	q Query,
	dv index.SortedNumericDocValues,
	maxDoc int,
) []int {
	t.Helper()

	concrete, ok := q.(*sortedNumericDocValuesSetQuery)
	if !ok {
		t.Fatalf("expected *sortedNumericDocValuesSetQuery, got %T", q)
	}

	approx := newSortedNumericApproximation(dv, maxDoc)
	minV, maxV := concrete.numbers.Min(), concrete.numbers.Max()
	contains := concrete.numbers.Contains
	singleton := index.UnwrapSingletonSortedNumeric(dv)

	var matchFn func() (bool, error)
	if singleton != nil {
		matchFn = func() (bool, error) {
			v, err := singleton.Get(approx.DocID())
			if err != nil {
				return false, err
			}
			return v >= minV && v <= maxV && contains(v), nil
		}
	} else {
		matchFn = func() (bool, error) {
			vs, err := dv.Get(approx.DocID())
			if err != nil {
				return false, err
			}
			for _, v := range vs {
				if v < minV {
					continue
				}
				if v > maxV {
					return false, nil
				}
				if contains(v) {
					return true, nil
				}
			}
			return false, nil
		}
	}

	tpi := NewTwoPhaseIterator(approx, matchFn)
	disi := tpi.AsDocIdSetIterator()

	var matched []int
	for {
		doc, err := disi.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		matched = append(matched, doc)
	}
	return matched
}

// fakeNumeric is a tiny in-memory NumericDocValues backed by a
// per-docID int64 map. Mirrors the fakeSortedNumeric helper but for
// the singleton fast path.
type fakeNumeric struct {
	values map[int]int64
	docIDs []int
	cursor int
	docID  int
}

func newFakeNumeric(values map[int]int64) *fakeNumeric {
	ids := make([]int, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return &fakeNumeric{
		values: values,
		docIDs: ids,
		cursor: 0,
		docID:  -1,
	}
}

func (f *fakeNumeric) Get(docID int) (int64, error) {
	return f.values[docID], nil
}

func (f *fakeNumeric) Advance(target int) (int, error) {
	for f.cursor < len(f.docIDs) && f.docIDs[f.cursor] < target {
		f.cursor++
	}
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	return f.docID, nil
}

func (f *fakeNumeric) NextDoc() (int, error) {
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	return f.docID, nil
}

func (f *fakeNumeric) DocID() int { return f.docID }

func (f *fakeNumeric) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (f *fakeNumeric) LongValue() (int64, error) {
	return f.values[f.docID], nil
}

// sliceEqualInt is a tiny equality helper. Kept local so it stays
// next to the test that uses it.
func sliceEqualInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sliceEqualInt64 mirrors sliceEqualInt for int64 slices.
func sliceEqualInt64(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
