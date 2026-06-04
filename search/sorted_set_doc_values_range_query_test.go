// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// br is a tiny helper that builds a *util.BytesRef from an ASCII
// string. Keeps the table-driven tests below readable.
func br(s string) *util.BytesRef {
	return util.NewBytesRef([]byte(s))
}

// TestNewSortedSetDocValuesRangeQuery_EmptyField rejects an empty
// field, mirroring the Java NPE on a null field. Returns an idiomatic
// error so callers can branch on it without panic semantics.
func TestNewSortedSetDocValuesRangeQuery_EmptyField(t *testing.T) {
	t.Parallel()
	_, err := NewSortedSetDocValuesRangeQuery("", br("a"), br("z"), true, true)
	if !errors.Is(err, errSortedSetDocValuesRangeQueryEmptyField) {
		t.Fatalf("empty field: got %v, want errSortedSetDocValuesRangeQueryEmptyField", err)
	}
}

// TestNewSortedSetDocValuesRangeQuery_InclusiveFoldsForNilBound covers
// the Java constructor's invariant: a null bound can never be
// inclusive. The constructor folds the inclusive flag to false in that
// case so the open-range form ("*" on one side) does not become a
// type-confused inclusive range.
func TestNewSortedSetDocValuesRangeQuery_InclusiveFoldsForNilBound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                     string
		lower, upper             *util.BytesRef
		lowerIn, upperIn         bool
		wantLowerIn, wantUpperIn bool
	}{
		{"nil-lower-inclusive-folds-false", nil, br("z"), true, true, false, true},
		{"nil-upper-inclusive-folds-false", br("a"), nil, true, true, true, false},
		{"both-nil-both-fold-false", nil, nil, true, true, false, false},
		{"non-nil-bounds-keep-flags", br("a"), br("z"), true, false, true, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedSetDocValuesRangeQuery("f", tc.lower, tc.upper, tc.lowerIn, tc.upperIn)
			if err != nil {
				t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
			}
			c := q.(*sortedSetDocValuesRangeQuery)
			if c.LowerInclusive() != tc.wantLowerIn {
				t.Errorf("LowerInclusive = %v, want %v", c.LowerInclusive(), tc.wantLowerIn)
			}
			if c.UpperInclusive() != tc.wantUpperIn {
				t.Errorf("UpperInclusive = %v, want %v", c.UpperInclusive(), tc.wantUpperIn)
			}
		})
	}
}

// TestSortedSetDocValuesRangeQuery_RewriteOpenRangeFoldsToFieldExists
// covers the rewrite() fast path from the Java reference: nil lower
// and nil upper together fold to FieldExistsQuery.
func TestSortedSetDocValuesRangeQuery_RewriteOpenRangeFoldsToFieldExists(t *testing.T) {
	t.Parallel()
	q, err := NewSortedSetDocValuesRangeQuery("f", nil, nil, false, false)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
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

// TestSortedSetDocValuesRangeQuery_RewriteBoundedRewritesToSelf covers
// the negative side of the rewrite() fast path: any range with at
// least one non-nil bound returns the query unchanged.
func TestSortedSetDocValuesRangeQuery_RewriteBoundedRewritesToSelf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		lower, upper *util.BytesRef
	}{
		{"both-bounds", br("a"), br("z")},
		{"only-lower", br("a"), nil},
		{"only-upper", nil, br("z")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedSetDocValuesRangeQuery("f", tc.lower, tc.upper, true, true)
			if err != nil {
				t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
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

// TestSortedSetDocValuesRangeQuery_Equals covers the equals contract:
// same class, same field, same bounds, same inclusivity flags.
func TestSortedSetDocValuesRangeQuery_Equals(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, false)
	b, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, false)
	cDiffField, _ := NewSortedSetDocValuesRangeQuery("g", br("a"), br("z"), true, false)
	cDiffLower, _ := NewSortedSetDocValuesRangeQuery("f", br("b"), br("z"), true, false)
	cDiffUpper, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("y"), true, false)
	cDiffLowerIn, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), false, false)
	cDiffUpperIn, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, true)

	if !a.Equals(b) {
		t.Error("a should equal b (identical fields)")
	}
	if !b.Equals(a) {
		t.Error("equals must be symmetric")
	}
	if a.Equals(cDiffField) {
		t.Error("a should not equal cDiffField")
	}
	if a.Equals(cDiffLower) {
		t.Error("a should not equal cDiffLower")
	}
	if a.Equals(cDiffUpper) {
		t.Error("a should not equal cDiffUpper")
	}
	if a.Equals(cDiffLowerIn) {
		t.Error("a should not equal cDiffLowerIn")
	}
	if a.Equals(cDiffUpperIn) {
		t.Error("a should not equal cDiffUpperIn")
	}
	if a.Equals(NewMatchNoDocsQuery()) {
		t.Error("a should not equal MatchNoDocsQuery (different type)")
	}
}

// TestSortedSetDocValuesRangeQuery_HashCodeStability covers the
// HashCode contract: equal queries share a hash; differences in field,
// either bound or either inclusivity flag should not collide.
func TestSortedSetDocValuesRangeQuery_HashCodeStability(t *testing.T) {
	t.Parallel()
	a, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, false)
	b, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, false)
	diffField, _ := NewSortedSetDocValuesRangeQuery("g", br("a"), br("z"), true, false)
	diffLower, _ := NewSortedSetDocValuesRangeQuery("f", br("b"), br("z"), true, false)
	diffUpper, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("y"), true, false)
	diffLowerIn, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), false, false)
	diffUpperIn, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, true)

	if a.HashCode() != b.HashCode() {
		t.Errorf("equal queries must share hash: %d vs %d", a.HashCode(), b.HashCode())
	}
	// Sensitivity checks. These are not strict requirements of the
	// hash contract, but they verify the fold structure keeps the
	// hash useful for hash-based collections.
	for _, other := range []Query{diffField, diffLower, diffUpper, diffLowerIn, diffUpperIn} {
		if a.HashCode() == other.HashCode() {
			t.Errorf("hash collided across distinct queries: %d (a) == %d", a.HashCode(), other.HashCode())
		}
	}
}

// TestSortedSetDocValuesRangeQuery_String covers toString(String):
// bracket choice follows inclusivity; '*' renders for nil bounds; the
// field prefix is elided when the default-field argument matches. Each
// bound is rendered with org.apache.lucene.util.BytesRef.toString()'s
// space-separated hex form ("[61]" for the byte 0x61 = 'a'), matching
// Lucene's SortedSetDocValuesRangeQuery.toString exactly.
func TestSortedSetDocValuesRangeQuery_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		lower, upper *util.BytesRef
		lIn, uIn     bool
		field        string
		defField     string
		want         string
	}{
		{"both-inclusive", br("a"), br("z"), true, true, "f", "other", "f:[[61] TO [7a]]"},
		{"both-exclusive", br("a"), br("z"), false, false, "f", "other", "f:{[61] TO [7a]}"},
		{"mixed", br("a"), br("z"), true, false, "f", "other", "f:[[61] TO [7a]}"},
		{"open-lower", nil, br("z"), false, true, "f", "other", "f:{* TO [7a]]"},
		{"open-upper", br("a"), nil, true, false, "f", "other", "f:[[61] TO *}"},
		{"both-open", nil, nil, false, false, "f", "other", "f:{* TO *}"},
		{"field-elided", br("a"), br("z"), true, true, "f", "f", "[[61] TO [7a]]"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedSetDocValuesRangeQuery(tc.field, tc.lower, tc.upper, tc.lIn, tc.uIn)
			if err != nil {
				t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
			}
			got := q.(*sortedSetDocValuesRangeQuery).String(tc.defField)
			if got != tc.want {
				t.Errorf("String(%q) = %q, want %q", tc.defField, got, tc.want)
			}
		})
	}
}

// TestSortedSetDocValuesRangeQuery_Visit covers the QueryVisitor
// dispatch: VisitLeaf is invoked iff AcceptField returns true.
func TestSortedSetDocValuesRangeQuery_Visit(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, true)

	accept := &sortedSetRangeVisitor{accept: true}
	q.(*sortedSetDocValuesRangeQuery).Visit(accept)
	if !accept.leafCalled {
		t.Error("VisitLeaf should be called when AcceptField returns true")
	}

	reject := &sortedSetRangeVisitor{accept: false}
	q.(*sortedSetDocValuesRangeQuery).Visit(reject)
	if reject.leafCalled {
		t.Error("VisitLeaf should not be called when AcceptField returns false")
	}
}

// sortedSetRangeVisitor is a tiny QueryVisitor used by the Visit test.
// Embeds EmptyQueryVisitorBase so the test only overrides the two
// hooks it exercises.
type sortedSetRangeVisitor struct {
	EmptyQueryVisitorBase
	accept     bool
	leafCalled bool
}

func (v *sortedSetRangeVisitor) AcceptField(_ string) bool { return v.accept }
func (v *sortedSetRangeVisitor) VisitLeaf(_ Query)         { v.leafCalled = true }

// TestSortedSetDocValuesRangeQuery_Clone confirms Clone returns the
// same query instance. The struct is logically immutable, so the
// shallow clone preserves identity (per the Java semantics of
// returning self when no transformation is necessary).
func TestSortedSetDocValuesRangeQuery_Clone(t *testing.T) {
	t.Parallel()
	q, _ := NewSortedSetDocValuesRangeQuery("f", br("a"), br("z"), true, true)
	if c := q.Clone(); c != q {
		t.Errorf("Clone() = %p, want %p (same instance)", c, q)
	}
}

// TestLookupTermLocal_HitsAndMisses pins the in-place binary-search
// substitute for the missing SortedSetDocValues.LookupTerm. The
// "negative => -insertionPoint - 1" sentinel must match the Java
// convention exactly so the caller-side arithmetic in resolveOrdRange
// stays correct.
func TestLookupTermLocal_HitsAndMisses(t *testing.T) {
	t.Parallel()
	// Terms: ["b", "d", "f"] => ords 0, 1, 2.
	dv := newFakeSortedSet(nil, []string{"b", "d", "f"})

	cases := []struct {
		name string
		term string
		want int
	}{
		{"hit-first", "b", 0},
		{"hit-middle", "d", 1},
		{"hit-last", "f", 2},
		// Misses return -(insertionPoint) - 1. Insertion point is
		// the index at which the term would be inserted to keep
		// the dictionary sorted.
		{"miss-before-all", "a", -1}, // -(0)-1
		{"miss-between-bd", "c", -2}, // -(1)-1
		{"miss-between-df", "e", -3}, // -(2)-1
		{"miss-after-all", "g", -4},  // -(3)-1
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := lookupTermLocal(dv, br(tc.term), dv.GetValueCount())
			if err != nil {
				t.Fatalf("lookupTermLocal: %v", err)
			}
			if got != tc.want {
				t.Errorf("lookupTermLocal(%q) = %d, want %d", tc.term, got, tc.want)
			}
		})
	}
}

// TestSortedSetDocValuesRangeQuery_End2End_MultiOrd drives the full
// query through a fake SortedSetDocValues and verifies the matching
// doc set across bound-inclusivity combinations. This is the closest
// analogue we have to the Java IndexSearcher-based peer for this
// query (no peer exists in Lucene 10.4.0).
func TestSortedSetDocValuesRangeQuery_End2End_MultiOrd(t *testing.T) {
	t.Parallel()

	// Dictionary: ["alpha", "bravo", "charlie", "delta", "echo"]
	// => ords         0,       1,        2,         3,       4.
	// Per-doc ords: 0 carries {alpha}, 1 carries {bravo,charlie},
	// 2 carries {delta}, 3 carries no values, 4 carries {echo}.
	terms := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	docs := map[int][]int{
		0: {0},
		1: {1, 2},
		2: {3},
		3: nil,
		4: {4},
	}

	cases := []struct {
		name         string
		lower, upper *util.BytesRef
		lIn, uIn     bool
		want         []int
	}{
		{"inclusive-bravo-to-delta", br("bravo"), br("delta"), true, true, []int{1, 2}},
		{"exclusive-bravo-to-delta", br("bravo"), br("delta"), false, false, []int{1}},
		{"miss-lower-bb-to-delta-inclusive", br("bb"), br("delta"), true, true, []int{1, 2}},
		{"miss-upper-bravo-to-de-exclusive-upper", br("bravo"), br("de"), true, false, []int{1}},
		{"open-lower-nil-to-bravo-inclusive", nil, br("bravo"), false, true, []int{0, 1}},
		{"open-upper-charlie-to-nil-inclusive", br("charlie"), nil, true, false, []int{1, 2, 4}},
		{"empty-window-zz-to-aa", br("zz"), br("aaa"), true, true, nil},
		{"only-alpha", br("alpha"), br("alpha"), true, true, []int{0}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewSortedSetDocValuesRangeQuery("f", tc.lower, tc.upper, tc.lIn, tc.uIn)
			if err != nil {
				t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
			}
			dv := newFakeSortedSet(docs, terms)
			got := runSortedSetRangeQueryWithDV(t, q, dv, 5)
			if !sliceEqualInt(got, tc.want) {
				t.Errorf("matched docs = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSortedSetDocValuesRangeQuery_End2End_SingletonFastPath drives
// the singleton-backed scorer branch. The Java reference unwraps
// SortedSet -> Sorted for fields with at most one ordinal per doc and
// uses singleton.ordValue() in matches(); Gocene mirrors that via
// index.UnwrapSingletonSortedSet / index.SingletonSortedSet.
func TestSortedSetDocValuesRangeQuery_End2End_SingletonFastPath(t *testing.T) {
	t.Parallel()

	// Dictionary: ["a","b","c","d","e"] (ords 0..4). One ord per doc.
	terms := []string{"a", "b", "c", "d", "e"}
	docs := map[int]int{
		0: 0, // a
		1: 1, // b
		2: 2, // c
		3: 3, // d
		4: 4, // e
	}
	sorted := newFakeSortedSingle(docs, terms)
	dv := index.SingletonSortedSet(sorted)

	q, err := NewSortedSetDocValuesRangeQuery("f", br("b"), br("d"), true, true)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
	}

	got := runSortedSetRangeQueryWithDV(t, q, dv, 5)
	want := []int{1, 2, 3}
	if !sliceEqualInt(got, want) {
		t.Errorf("matched docs = %v, want %v", got, want)
	}
}

// TestSortedSetDocValuesRangeQuery_End2End_EmptyWindow_NoMatches
// covers the "min > max" fast path: when the resolved ordinal window
// is empty (no overlap with the dictionary), the supplier yields an
// empty iterator and the scorer matches zero documents.
func TestSortedSetDocValuesRangeQuery_End2End_EmptyWindow_NoMatches(t *testing.T) {
	t.Parallel()
	terms := []string{"alpha", "bravo"}
	docs := map[int][]int{0: {0}, 1: {1}}
	q, err := NewSortedSetDocValuesRangeQuery("f", br("zz"), br("zzz"), true, true)
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesRangeQuery: %v", err)
	}
	dv := newFakeSortedSet(docs, terms)
	got := runSortedSetRangeQueryWithDV(t, q, dv, 2)
	if len(got) != 0 {
		t.Errorf("matched docs = %v, want []", got)
	}
}

// runSortedSetRangeQueryWithDV replays the supplier branch from
// CreateWeight against a pre-built SortedSetDocValues iterator and a
// known maxDoc, returning the matched doc ids in ascending order.
// Mirrors runSortedNumericSetQueryWithDV in the sister test file so
// the two tests stay structurally similar.
func runSortedSetRangeQueryWithDV(
	t *testing.T,
	q Query,
	dv index.SortedSetDocValues,
	maxDoc int,
) []int {
	t.Helper()

	concrete, ok := q.(*sortedSetDocValuesRangeQuery)
	if !ok {
		t.Fatalf("expected *sortedSetDocValuesRangeQuery, got %T", q)
	}

	minOrd, maxOrd, err := concrete.resolveOrdRange(dv)
	if err != nil {
		t.Fatalf("resolveOrdRange: %v", err)
	}
	if minOrd > maxOrd {
		return nil
	}

	approx := newSortedSetApproximation(dv, maxDoc)
	singleton := index.UnwrapSingletonSortedSet(dv)

	var matchFn func() (bool, error)
	if singleton != nil {
		matchFn = func() (bool, error) {
			ord, err := singleton.OrdValue()
			if err != nil {
				return false, err
			}
			if ord < 0 {
				return false, nil
			}
			return ord >= minOrd && ord <= maxOrd, nil
		}
	} else {
		matchFn = func() (bool, error) {
			for {
				ord, err := dv.NextOrd()
				if err != nil {
					return false, err
				}
				if ord == -1 {
					break
				}
				if ord < minOrd {
					continue
				}
				return ord <= maxOrd, nil
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

// fakeSortedSet is a tiny in-memory SortedSetDocValues backed by a
// per-docID slice of ordinals and a flat term dictionary. Mirrors the
// fakeSortedNumeric pattern from lat_lon_doc_values_query_test.go.
type fakeSortedSet struct {
	docs   map[int][]int
	terms  []string
	docIDs []int
	cursor int
	docID  int
	ordIdx int
}

func newFakeSortedSet(docs map[int][]int, terms []string) *fakeSortedSet {
	ids := make([]int, 0, len(docs))
	for id, ords := range docs {
		// Skip docs with no ordinals; the iterator must only stop on
		// docs that carry at least one value (matches the Java
		// contract).
		if len(ords) == 0 {
			continue
		}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return &fakeSortedSet{
		docs:   docs,
		terms:  terms,
		docIDs: ids,
		cursor: 0,
		docID:  -1,
	}
}

func (f *fakeSortedSet) Cost() int64 { return int64(len(f.docIDs)) }

func (f *fakeSortedSet) Advance(target int) (int, error) {
	for f.cursor < len(f.docIDs) && f.docIDs[f.cursor] < target {
		f.cursor++
	}
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	f.ordIdx = 0
	return f.docID, nil
}

func (f *fakeSortedSet) NextDoc() (int, error) {
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	f.ordIdx = 0
	return f.docID, nil
}

func (f *fakeSortedSet) DocID() int { return f.docID }

func (f *fakeSortedSet) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (f *fakeSortedSet) NextOrd() (int, error) {
	if f.docID < 0 || f.docID == NO_MORE_DOCS {
		return -1, nil
	}
	ords := f.docs[f.docID]
	if f.ordIdx >= len(ords) {
		return -1, nil
	}
	o := ords[f.ordIdx]
	f.ordIdx++
	return o, nil
}

func (f *fakeSortedSet) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(f.terms) {
		return nil, nil
	}
	return []byte(f.terms[ord]), nil
}

func (f *fakeSortedSet) GetValueCount() int { return len(f.terms) }

// fakeSortedSingle is a tiny SortedDocValues backed by a per-docID
// ordinal map and the same flat term dictionary as fakeSortedSet. Used
// by the singleton fast-path test to feed index.SingletonSortedSet.
type fakeSortedSingle struct {
	ords   map[int]int
	terms  []string
	docIDs []int
	cursor int
	docID  int
}

func newFakeSortedSingle(ords map[int]int, terms []string) *fakeSortedSingle {
	ids := make([]int, 0, len(ords))
	for id := range ords {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return &fakeSortedSingle{
		ords:   ords,
		terms:  terms,
		docIDs: ids,
		cursor: 0,
		docID:  -1,
	}
}

// getInternal serves the random-access lookup used by BinaryValue.
func (f *fakeSortedSingle) getInternal(docID int) ([]byte, error) {
	ord, ok := f.ords[docID]
	if !ok {
		return nil, nil
	}
	if ord < 0 || ord >= len(f.terms) {
		return nil, nil
	}
	return []byte(f.terms[ord]), nil
}

func (f *fakeSortedSingle) Cost() int64 { return int64(len(f.docIDs)) }

func (f *fakeSortedSingle) LongValue() (int64, error) {
	if f.docID < 0 || f.docID == NO_MORE_DOCS {
		return -1, nil
	}
	if ord, ok := f.ords[f.docID]; ok {
		return int64(ord), nil
	}
	return -1, nil
}

func (f *fakeSortedSingle) Advance(target int) (int, error) {
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

func (f *fakeSortedSingle) NextDoc() (int, error) {
	if f.cursor >= len(f.docIDs) {
		f.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	f.docID = f.docIDs[f.cursor]
	f.cursor++
	return f.docID, nil
}

func (f *fakeSortedSingle) DocID() int { return f.docID }

func (f *fakeSortedSingle) AdvanceExact(target int) (bool, error) {
	got, err := f.Advance(target)
	if err != nil {
		return false, err
	}
	return got == target, nil
}

func (f *fakeSortedSingle) BinaryValue() ([]byte, error) {
	if f.docID < 0 || f.docID == NO_MORE_DOCS {
		return nil, nil
	}
	return f.getInternal(f.docID)
}

func (f *fakeSortedSingle) OrdValue() (int, error) {
	if f.docID < 0 || f.docID == NO_MORE_DOCS {
		return -1, nil
	}
	if ord, ok := f.ords[f.docID]; ok {
		return ord, nil
	}
	return -1, nil
}

func (f *fakeSortedSingle) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(f.terms) {
		return nil, nil
	}
	return []byte(f.terms[ord]), nil
}

func (f *fakeSortedSingle) GetValueCount() int { return len(f.terms) }
