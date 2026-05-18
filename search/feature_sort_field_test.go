// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestFeatureSortField_Constructor_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()

	if _, err := NewFeatureSortField("", "pagerank"); err == nil {
		t.Fatalf("expected error for empty field, got nil")
	}
	if _, err := NewFeatureSortField("features", ""); err == nil {
		t.Fatalf("expected error for empty featureName, got nil")
	}
}

func TestFeatureSortField_Constructor_DefaultsReverseTrueAndCustomType(t *testing.T) {
	t.Parallel()

	fsf, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	if fsf.SortField == nil {
		t.Fatalf("embedded SortField must not be nil")
	}
	if fsf.SortField.Field != "features" {
		t.Errorf("Field = %q, want %q", fsf.SortField.Field, "features")
	}
	if fsf.FeatureName() != "pagerank" {
		t.Errorf("FeatureName() = %q, want %q", fsf.FeatureName(), "pagerank")
	}
	if !fsf.SortField.Reverse {
		t.Errorf("Reverse must default to true (higher feature values rank first)")
	}
	if fsf.SortField.Type != SortFieldTypeCustom {
		t.Errorf("Type = %v, want SortFieldTypeCustom", fsf.SortField.Type)
	}
}

func TestFeatureSortField_Equals_HashCode(t *testing.T) {
	t.Parallel()

	a, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor a: %v", err)
	}
	b, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor b: %v", err)
	}
	c, err := NewFeatureSortField("features", "urlauth")
	if err != nil {
		t.Fatalf("ctor c: %v", err)
	}
	d, err := NewFeatureSortField("other", "pagerank")
	if err != nil {
		t.Fatalf("ctor d: %v", err)
	}

	if !a.Equals(b) {
		t.Errorf("expected a.Equals(b) for matching field+featureName")
	}
	if a.HashCode() != b.HashCode() {
		t.Errorf("HashCode must be consistent for equal values: %d vs %d", a.HashCode(), b.HashCode())
	}
	if a.Equals(c) {
		t.Errorf("featureName mismatch must not compare equal")
	}
	if a.Equals(d) {
		t.Errorf("field mismatch must not compare equal")
	}
	if a.Equals(nil) {
		t.Errorf("nil comparison must be false")
	}
	if !a.Equals(a) {
		t.Errorf("reflexive equality must hold")
	}
}

func TestFeatureSortField_SetMissingValue_Rejected(t *testing.T) {
	t.Parallel()

	fsf, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	err = fsf.SetMissingValue(float32(1.5))
	if err == nil {
		t.Fatalf("expected error from SetMissingValue")
	}
	if !errors.Is(err, ErrFeatureSortFieldMissingValueUnsupported) {
		t.Errorf("expected ErrFeatureSortFieldMissingValueUnsupported sentinel, got %v", err)
	}
}

func TestFeatureSortField_String(t *testing.T) {
	t.Parallel()

	fsf, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	got := fsf.String()
	want := `<feature:"features" featureName=pagerank>`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFeatureSortField_SortWrapper_DoesNotNeedScores(t *testing.T) {
	t.Parallel()

	fsf, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	sort := NewSort(fsf.SortField)
	if sort.NeedsScores() {
		t.Errorf("FeatureSortField wrapped in a Sort must not require scores")
	}
}

func TestFeatureSortField_GetComparator_NumHits(t *testing.T) {
	t.Parallel()

	fsf, err := NewFeatureSortField("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	cmp := fsf.GetComparator(8, PruningNone)
	if cmp == nil {
		t.Fatalf("GetComparator returned nil")
	}
	if got := len(cmp.values); got != 8 {
		t.Errorf("values slot count = %d, want 8", got)
	}
	if cmp.field != "features" {
		t.Errorf("field = %q, want features", cmp.field)
	}
	if cmp.featureTerm == nil || cmp.featureTerm.Text() != "pagerank" {
		t.Errorf("feature term not initialised correctly: %+v", cmp.featureTerm)
	}
}

// fakeFeaturePostings drives FeatureComparator with a predefined doc -> freq
// mapping. It implements just enough of index.PostingsEnum for getValueForDoc.
type fakeFeaturePostings struct {
	docs  []int
	freqs []int
	idx   int // -1 before first NextDoc/Advance
}

func newFakeFeaturePostings(docs []int, freqs []int) *fakeFeaturePostings {
	if len(docs) != len(freqs) {
		panic("fakeFeaturePostings: docs and freqs length mismatch")
	}
	return &fakeFeaturePostings{docs: docs, freqs: freqs, idx: -1}
}

func (f *fakeFeaturePostings) NextDoc() (int, error) {
	f.idx++
	if f.idx >= len(f.docs) {
		f.idx = len(f.docs)
		return index.NO_MORE_DOCS, nil
	}
	return f.docs[f.idx], nil
}

func (f *fakeFeaturePostings) Advance(target int) (int, error) {
	if f.idx < 0 {
		f.idx = 0
	}
	for f.idx < len(f.docs) && f.docs[f.idx] < target {
		f.idx++
	}
	if f.idx >= len(f.docs) {
		f.idx = len(f.docs)
		return index.NO_MORE_DOCS, nil
	}
	return f.docs[f.idx], nil
}

func (f *fakeFeaturePostings) DocID() int {
	if f.idx < 0 {
		return -1
	}
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS
	}
	return f.docs[f.idx]
}

func (f *fakeFeaturePostings) Freq() (int, error) {
	if f.idx < 0 || f.idx >= len(f.docs) {
		return 0, nil
	}
	return f.freqs[f.idx], nil
}

func (f *fakeFeaturePostings) NextPosition() (int, error)  { return index.NO_MORE_POSITIONS, nil }
func (f *fakeFeaturePostings) StartOffset() (int, error)   { return -1, nil }
func (f *fakeFeaturePostings) EndOffset() (int, error)     { return -1, nil }
func (f *fakeFeaturePostings) GetPayload() ([]byte, error) { return nil, nil }
func (f *fakeFeaturePostings) Cost() int64                 { return int64(len(f.docs)) }

func TestFeatureComparator_GetValueForDoc_DecodesFreq(t *testing.T) {
	t.Parallel()

	cmp := NewFeatureComparator(4, "features", "pagerank")

	values := []float32{0.25, 4.0, 17.5}
	docs := []int{0, 3, 7}
	freqs := make([]int, len(values))
	for i, v := range values {
		freqs[i] = int(document.EncodeFeatureValueAsTermFreq(v))
	}
	cmp.currentReaderPostingsValues = newFakeFeaturePostings(docs, freqs)

	cases := []struct {
		doc  int
		want float32
	}{
		{doc: 0, want: values[0]},
		{doc: 1, want: 0},
		{doc: 3, want: values[1]},
		{doc: 5, want: 0},
		{doc: 7, want: values[2]},
		{doc: 9, want: 0},
	}
	for _, tc := range cases {
		got, err := cmp.getValueForDoc(tc.doc)
		if err != nil {
			t.Fatalf("getValueForDoc(%d) error: %v", tc.doc, err)
		}
		// Decode is not exact in single-precision; accept small relative drift.
		if !approxEqualFloat32(got, tc.want, 1e-6) {
			t.Errorf("getValueForDoc(%d) = %v, want approx %v", tc.doc, got, tc.want)
		}
	}
}

func TestFeatureComparator_CopyCompareSequence(t *testing.T) {
	t.Parallel()

	cmp := NewFeatureComparator(4, "features", "pagerank")
	docs := []int{0, 1, 2, 3}
	values := []float32{1.0, 4.0, 2.0, 8.0}
	freqs := make([]int, len(values))
	for i, v := range values {
		freqs[i] = int(document.EncodeFeatureValueAsTermFreq(v))
	}
	cmp.currentReaderPostingsValues = newFakeFeaturePostings(docs, freqs)

	for slot, doc := range docs {
		if err := cmp.Copy(slot, doc); err != nil {
			t.Fatalf("Copy(slot=%d, doc=%d): %v", slot, doc, err)
		}
	}

	// values slot 1 (4.0) > slot 0 (1.0)
	if got := cmp.Compare(0, 1); got >= 0 {
		t.Errorf("Compare(0,1) = %d, want negative (1.0 < 4.0)", got)
	}
	if got := cmp.Compare(3, 0); got <= 0 {
		t.Errorf("Compare(3,0) = %d, want positive (8.0 > 1.0)", got)
	}
	if got := cmp.Compare(2, 2); got != 0 {
		t.Errorf("Compare(2,2) = %d, want 0", got)
	}

	// CompareTop with topValue equal to the best slot's value rounds to zero.
	cmp.SetTopValue(cmp.Value(3))
	cmp2, err := cmp.CompareTop(3)
	if err != nil {
		t.Fatalf("CompareTop: %v", err)
	}
	if cmp2 != 0 {
		t.Errorf("CompareTop equal-values = %d, want 0", cmp2)
	}

	// SetBottom on the weakest slot makes a stronger doc compare negative
	// (bottom is worse than doc).
	cmp.SetBottom(0) // bottom = 1.0
	cmpB, err := cmp.CompareBottom(3)
	if err != nil {
		t.Fatalf("CompareBottom: %v", err)
	}
	if cmpB >= 0 {
		t.Errorf("CompareBottom against stronger doc = %d, want negative", cmpB)
	}
}

func TestFeatureComparator_NoPostings_ReturnsZero(t *testing.T) {
	t.Parallel()

	cmp := NewFeatureComparator(2, "features", "pagerank")
	cmp.currentReaderPostingsValues = nil

	got, err := cmp.getValueForDoc(0)
	if err != nil {
		t.Fatalf("getValueForDoc: %v", err)
	}
	if got != 0 {
		t.Errorf("missing postings must yield 0, got %v", got)
	}
}

func TestFeatureComparator_DoSetNextReader_NilContextErrors(t *testing.T) {
	t.Parallel()

	cmp := NewFeatureComparator(1, "features", "pagerank")
	err := cmp.DoSetNextReader(nil)
	if err == nil {
		t.Fatalf("expected error on nil context")
	}
	if !strings.Contains(err.Error(), "leaf reader context") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCompareFloat32_Semantics(t *testing.T) {
	t.Parallel()

	nan := float32(math.NaN())

	cases := []struct {
		name string
		a, b float32
		want int
	}{
		{"less", 1.0, 2.0, -1},
		{"greater", 5.5, 1.5, 1},
		{"equal", 3.14, 3.14, 0},
		{"nan_vs_value", nan, 1.0, 1},
		{"value_vs_nan", 1.0, nan, -1},
		{"nan_vs_nan", nan, nan, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareFloat32(tc.a, tc.b); got != tc.want {
				t.Errorf("compareFloat32(%v,%v) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func approxEqualFloat32(a, b, tolerance float32) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	max := b
	if max < 0 {
		max = -max
	}
	return diff <= tolerance*(1+max)
}
