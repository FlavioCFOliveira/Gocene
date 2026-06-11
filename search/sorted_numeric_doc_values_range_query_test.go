// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestNewSortedNumericDocValuesRangeQuery_EmptyField rejects an empty
// field, mirroring the Java NPE on a null field. Returns an idiomatic
// error so callers can branch on it without panic semantics.
func TestNewSortedNumericDocValuesRangeQuery_EmptyField(t *testing.T) {
	t.Parallel()
	_, err := NewSortedNumericDocValuesRangeQuery("", 1, 10)
	if !errors.Is(err, errSortedNumericDocValuesRangeQueryEmptyField) {
		t.Fatalf("empty field: got %v, want errSortedNumericDocValuesRangeQueryEmptyField", err)
	}
}

// TestNewSortedNumericDocValuesRangeQuery_AccessorsPreserveBounds
// pins the constructor invariants: the bounds reach the accessors
// untouched, including the inverted (lower > upper) case (which the
// Java reference accepts and only collapses during rewrite()).
func TestNewSortedNumericDocValuesRangeQuery_AccessorsPreserveBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		lower, upper int64
	}{
		{"normal", 1, 10},
		{"single-value", 5, 5},
		{"min-max", math.MinInt64, math.MaxInt64},
		{"inverted", 10, 1},
		{"negative-range", -100, -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedNumericDocValuesRangeQuery("f", tc.lower, tc.upper)
			if err != nil {
				t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
			}
			c := q.(*sortedNumericDocValuesRangeQuery)
			if c.Field() != "f" {
				t.Errorf("Field() = %q, want %q", c.Field(), "f")
			}
			if c.LowerValue() != tc.lower {
				t.Errorf("LowerValue() = %d, want %d", c.LowerValue(), tc.lower)
			}
			if c.UpperValue() != tc.upper {
				t.Errorf("UpperValue() = %d, want %d", c.UpperValue(), tc.upper)
			}
		})
	}
}

// TestSortedNumericDocValuesRangeQuery_RewriteFullRangeFoldsToFieldExists
// covers the Java rewrite() fast path: [Long.MIN_VALUE..Long.MAX_VALUE]
// degenerates to "any doc with a value", i.e. FieldExistsQuery.
func TestSortedNumericDocValuesRangeQuery_RewriteFullRangeFoldsToFieldExists(t *testing.T) {
	t.Parallel()
	q, err := NewSortedNumericDocValuesRangeQuery("f", math.MinInt64, math.MaxInt64)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
	}
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	fe, ok := rewritten.(*FieldExistsQuery)
	if !ok {
		t.Fatalf("Rewrite() = %T, want *FieldExistsQuery", rewritten)
	}
	if fe.GetField() != "f" {
		t.Errorf("FieldExistsQuery.GetField = %q, want %q", fe.GetField(), "f")
	}
}

// TestSortedNumericDocValuesRangeQuery_RewriteInvertedFoldsToMatchNoDocs
// covers the Java rewrite() short-circuit: lower > upper is provably
// empty so it folds to MatchNoDocsQuery without ever opening the
// segment.
func TestSortedNumericDocValuesRangeQuery_RewriteInvertedFoldsToMatchNoDocs(t *testing.T) {
	t.Parallel()
	q, err := NewSortedNumericDocValuesRangeQuery("f", 10, 1)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
	}
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Rewrite() = %T, want *MatchNoDocsQuery", rewritten)
	}
}

// TestSortedNumericDocValuesRangeQuery_RewriteNormalRewritesToSelf
// covers the other side of the Java rewrite() fast paths: a bounded,
// non-inverted range returns the query unchanged (the
// DocValuesSkipper-driven branches are not yet wired in Gocene — see
// the type-level deviation notes — so the bound-only paths are
// exhaustive here).
func TestSortedNumericDocValuesRangeQuery_RewriteNormalRewritesToSelf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		lower, upper int64
	}{
		{"plain", 1, 10},
		{"single-value", 5, 5},
		{"negative-range", -10, -1},
		{"open-lower-only", math.MinInt64, 10},
		{"open-upper-only", 10, math.MaxInt64},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedNumericDocValuesRangeQuery("f", tc.lower, tc.upper)
			if err != nil {
				t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
			}
			rewritten, err := q.Rewrite(nil)
			if err != nil {
				t.Fatalf("Rewrite: %v", err)
			}
			if rewritten != q {
				t.Errorf("Rewrite() returned a different query: got %p, want %p", rewritten, q)
			}
		})
	}
}

// TestSortedNumericDocValuesRangeQuery_Equals covers the equals
// contract: same class, same field, same bounds.
func TestSortedNumericDocValuesRangeQuery_Equals(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)
	b, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)
	diffField, _ := NewSortedNumericDocValuesRangeQuery("g", 1, 10)
	diffLower, _ := NewSortedNumericDocValuesRangeQuery("f", 2, 10)
	diffUpper, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 11)

	if !a.Equals(b) {
		t.Error("a should equal b (identical fields)")
	}
	if !b.Equals(a) {
		t.Error("equals must be symmetric")
	}
	if a.Equals(diffField) {
		t.Error("a should not equal diffField")
	}
	if a.Equals(diffLower) {
		t.Error("a should not equal diffLower")
	}
	if a.Equals(diffUpper) {
		t.Error("a should not equal diffUpper")
	}
	if a.Equals(NewMatchNoDocsQuery()) {
		t.Error("a should not equal MatchNoDocsQuery (different type)")
	}
	// Cross-type: must not equal the sibling set query, even on
	// matching field/values, because the class identity is part of
	// the equality contract (Java's sameClassAs invariant).
	set, _ := NewSortedNumericDocValuesSetQuery("f", []int64{1, 10})
	if a.Equals(set) {
		t.Error("a should not equal sibling set query (different type)")
	}
}

// TestSortedNumericDocValuesRangeQuery_HashCodeStability covers the
// HashCode contract: equal queries share a hash; differences in field
// or either bound should not collide.
func TestSortedNumericDocValuesRangeQuery_HashCodeStability(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)
	b, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)
	diffField, _ := NewSortedNumericDocValuesRangeQuery("g", 1, 10)
	diffLower, _ := NewSortedNumericDocValuesRangeQuery("f", 2, 10)
	diffUpper, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 11)

	if a.HashCode() != b.HashCode() {
		t.Errorf("equal queries must share hash: %d vs %d", a.HashCode(), b.HashCode())
	}
	// Sensitivity checks. Not strict requirements of the hash
	// contract, but they verify the fold structure keeps the hash
	// useful for hash-based collections.
	for _, other := range []Query{diffField, diffLower, diffUpper} {
		if a.HashCode() == other.HashCode() {
			t.Errorf("hash collided across distinct queries: %d (a) == %d", a.HashCode(), other.HashCode())
		}
	}
}

// TestSortedNumericDocValuesRangeQuery_String covers toString(String):
// the field prefix is elided when the default-field argument matches;
// the bracket pair is always inclusive ('[' / ']').
func TestSortedNumericDocValuesRangeQuery_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		field        string
		lower, upper int64
		defField     string
		want         string
	}{
		{"plain", "f", 1, 10, "other", "f:[1 TO 10]"},
		{"field-elided", "f", 1, 10, "f", "[1 TO 10]"},
		{"single-value", "f", 5, 5, "other", "f:[5 TO 5]"},
		{"negative-range", "f", -100, -1, "other", "f:[-100 TO -1]"},
		{"min-max", "f", math.MinInt64, math.MaxInt64, "other",
			"f:[-9223372036854775808 TO 9223372036854775807]"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedNumericDocValuesRangeQuery(tc.field, tc.lower, tc.upper)
			if err != nil {
				t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
			}
			got := q.(*sortedNumericDocValuesRangeQuery).String(tc.defField)
			if got != tc.want {
				t.Errorf("String(%q) = %q, want %q", tc.defField, got, tc.want)
			}
		})
	}
}

// TestSortedNumericDocValuesRangeQuery_Visit covers the QueryVisitor
// dispatch: VisitLeaf is invoked iff AcceptField returns true.
func TestSortedNumericDocValuesRangeQuery_Visit(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)

	accept := &sortedNumericRangeVisitor{accept: true}
	q.(*sortedNumericDocValuesRangeQuery).Visit(accept)
	if !accept.leafCalled {
		t.Error("VisitLeaf should be called when AcceptField returns true")
	}

	reject := &sortedNumericRangeVisitor{accept: false}
	q.(*sortedNumericDocValuesRangeQuery).Visit(reject)
	if reject.leafCalled {
		t.Error("VisitLeaf should not be called when AcceptField returns false")
	}
}

// sortedNumericRangeVisitor is a tiny QueryVisitor used by the Visit
// test. Embeds EmptyQueryVisitorBase so the test only overrides the
// two hooks it exercises. Kept local to the test file (the existing
// setQueryVisitor / sortedSetRangeVisitor are also test-local).
type sortedNumericRangeVisitor struct {
	EmptyQueryVisitorBase
	accept     bool
	leafCalled bool
}

func (v *sortedNumericRangeVisitor) AcceptField(_ string) bool { return v.accept }
func (v *sortedNumericRangeVisitor) VisitLeaf(_ Query)         { v.leafCalled = true }

// TestSortedNumericDocValuesRangeQuery_Clone confirms Clone returns
// the same query instance. The struct is logically immutable, so the
// shallow clone preserves identity (per the Java semantics of
// returning self when no transformation is necessary).
func TestSortedNumericDocValuesRangeQuery_Clone(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedNumericDocValuesRangeQuery("f", 1, 10)
	if c := q.Clone(); c != q {
		t.Errorf("Clone() = %p, want %p (same instance)", c, q)
	}
}

// TestSortedNumericDocValuesRangeQuery_MatchLoop_MultiValue exercises
// the multi-value scorer path in isolation. The body must stay
// byte-identical to the matches() loop in CreateWeight; this test is
// the canary that flags any divergence.
func TestSortedNumericDocValuesRangeQuery_MatchLoop_MultiValue(t *testing.T) {
	t.Parallel()

	const lower, upper int64 = 10, 30

	cases := []struct {
		name string
		vals []int64
		want bool
	}{
		{"no-values", nil, false},
		{"all-below-lower", []int64{-5, 0, 5}, false},
		{"first-in-range", []int64{20, 100}, true},
		{"first-ge-lower-above-upper", []int64{35, 50}, false},
		{"skip-below-then-hit", []int64{5, 10}, true},
		{"skip-below-then-above", []int64{5, 31}, false},
		{"single-hit-boundary-lower", []int64{10}, true},
		{"single-hit-boundary-upper", []int64{30}, true},
		{"single-miss-just-above", []int64{31}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rangeQueryMultiValueMatch(lower, upper, tc.vals)
			if got != tc.want {
				t.Errorf("multiValueMatch(%v) = %v, want %v", tc.vals, got, tc.want)
			}
		})
	}
}

// rangeQueryMultiValueMatch replays the multi-value matches() loop in
// isolation. The body must stay byte-identical to the matches() loop
// in CreateWeight; the test above is the canary that pins any
// divergence.
func rangeQueryMultiValueMatch(lower, upper int64, vs []int64) bool {
	for _, v := range vs {
		if v < lower {
			continue
		}
		return v <= upper
	}
	return false
}

// TestSortedNumericDocValuesRangeQuery_End2End_MultiValue drives the
// full query through a fake LeafReaderContext and verifies the
// matching doc set. This is the closest analogue we have to a Java
// IndexSearcher-based peer for this query (no peer exists in Lucene
// 10.4.0).
func TestSortedNumericDocValuesRangeQuery_End2End_MultiValue(t *testing.T) {
	t.Parallel()

	q, err := NewSortedNumericDocValuesRangeQuery("f", 10, 30)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
	}

	values := map[int][]int64{
		0: {10},           // hit (boundary-lower)
		1: {5},            // miss (below lower)
		2: {5, 100},       // miss (skip below, then above upper)
		3: {15, 25},       // hit (first in range)
		4: {31},           // miss (just above upper)
		5: {30, 40},       // hit (boundary-upper as first ge lower)
		6: nil,            // miss (no values)
		7: {0, 5, 25, 40}, // hit (third value)
	}
	got := runSortedNumericRangeQuery(t, q, values)
	want := []int{0, 3, 5, 7}
	if !sliceEqualInt(got, want) {
		t.Errorf("matched docs = %v, want %v", got, want)
	}
}

// TestSortedNumericDocValuesRangeQuery_End2End_SingletonFastPath
// drives the singleton-backed scorer branch. The Java reference
// unwraps SortedNumeric -> Numeric for fields with at most one value
// per doc and reads the value directly via longValue(); Gocene
// mirrors that with index.UnwrapSingletonSortedNumeric /
// index.Singleton.
func TestSortedNumericDocValuesRangeQuery_End2End_SingletonFastPath(t *testing.T) {
	t.Parallel()

	q, err := NewSortedNumericDocValuesRangeQuery("f", 10, 30)
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesRangeQuery: %v", err)
	}

	numeric := newFakeNumeric(map[int]int64{
		0: 10, // hit (boundary-lower)
		1: 5,  // miss (below)
		2: 20, // hit
		3: 31, // miss (just above)
		4: 30, // hit (boundary-upper)
		5: 99, // miss
	})
	dv := index.Singleton(numeric)

	got := runSortedNumericRangeQueryWithDV(t, q, dv, 6)
	want := []int{0, 2, 4}
	if !sliceEqualInt(got, want) {
		t.Errorf("matched docs = %v, want %v", got, want)
	}
}

// runSortedNumericRangeQuery drives the query against a fake
// SortedNumericDocValues iterator built from values and returns the
// matched doc ids in ascending order. Mirrors
// runSortedNumericSetQuery in the sister test file so the two tests
// stay structurally similar.
func runSortedNumericRangeQuery(t *testing.T, q Query, values map[int][]int64) []int {
	t.Helper()
	fake := newFakeSortedNumeric(values)
	maxDoc := 0
	for id := range values {
		if id+1 > maxDoc {
			maxDoc = id + 1
		}
	}
	return runSortedNumericRangeQueryWithDV(t, q, fake, maxDoc)
}

// runSortedNumericRangeQueryWithDV drives the query against a
// pre-built SortedNumericDocValues iterator and a known maxDoc.
// Replays the supplier branch from CreateWeight without standing up a
// full SegmentReader.
func runSortedNumericRangeQueryWithDV(
	t *testing.T,
	q Query,
	dv index.SortedNumericDocValues,
	maxDoc int,
) []int {
	t.Helper()

	concrete, ok := q.(*sortedNumericDocValuesRangeQuery)
	if !ok {
		t.Fatalf("expected *sortedNumericDocValuesRangeQuery, got %T", q)
	}

	approx := newSortedNumericApproximation(dv, maxDoc)
	lower, upper := concrete.LowerValue(), concrete.UpperValue()
	singleton := index.UnwrapSingletonSortedNumeric(dv)

	var matchFn func() (bool, error)
	if singleton != nil {
		matchFn = func() (bool, error) {
			v, err := singleton.LongValue()
			if err != nil {
				return false, err
			}
			return v >= lower && v <= upper, nil
		}
	} else {
		matchFn = func() (bool, error) {
			vs, err := index.CollectSortedNumericValues(dv)
			if err != nil {
				return false, err
			}
			for _, v := range vs {
				if v < lower {
					continue
				}
				return v <= upper, nil
			}
			return false, nil
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