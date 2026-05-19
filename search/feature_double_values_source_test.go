// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

func TestFeatureDoubleValuesSource_Constructor_RejectsEmptyArgs(t *testing.T) {
	t.Parallel()

	if _, err := NewFeatureDoubleValuesSource("", "pagerank"); err == nil {
		t.Fatalf("expected error for empty field, got nil")
	}
	if _, err := NewFeatureDoubleValuesSource("features", ""); err == nil {
		t.Fatalf("expected error for empty featureName, got nil")
	}
}

func TestFeatureDoubleValuesSource_Constructor_StoresFieldAndFeature(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	if s.Field() != "features" {
		t.Errorf("Field() = %q, want %q", s.Field(), "features")
	}
	if s.FeatureName() != "pagerank" {
		t.Errorf("FeatureName() = %q, want %q", s.FeatureName(), "pagerank")
	}
	if s.featureTerm == nil || s.featureTerm.Text() != "pagerank" {
		t.Errorf("featureTerm not initialised correctly: %+v", s.featureTerm)
	}
	if s.featureTerm.Field != "features" {
		t.Errorf("featureTerm.Field = %q, want %q", s.featureTerm.Field, "features")
	}
}

func TestFeatureDoubleValuesSource_IsCacheable_AlwaysTrue(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if !s.IsCacheable(nil) {
		t.Errorf("IsCacheable(nil) = false, want true")
	}
}

func TestFeatureDoubleValuesSource_NeedsScores_AlwaysFalse(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if s.NeedsScores() {
		t.Errorf("NeedsScores() = true, want false")
	}
}

func TestFeatureDoubleValuesSource_Rewrite_ReturnsSelf(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if got := s.Rewrite(nil); got != s {
		t.Errorf("Rewrite(nil) = %p, want %p (self)", got, s)
	}
}

func TestFeatureDoubleValuesSource_GetValues_NilContextErrors(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if _, err := s.GetValues(nil, nil); err == nil {
		t.Errorf("expected error on nil context")
	}
}

func TestFeatureDoubleValuesSource_Equals_HashCode(t *testing.T) {
	t.Parallel()

	a, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor a: %v", err)
	}
	b, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor b: %v", err)
	}
	c, err := NewFeatureDoubleValuesSource("features", "urlauth")
	if err != nil {
		t.Fatalf("ctor c: %v", err)
	}
	d, err := NewFeatureDoubleValuesSource("other", "pagerank")
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
	if a.HashCode() == c.HashCode() {
		t.Errorf("HashCode should differ when featureName differs (collision check)")
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

	var nilSrc *FeatureDoubleValuesSource
	if nilSrc.Equals(a) {
		t.Errorf("nil receiver vs non-nil must be false")
	}
}

func TestFeatureDoubleValuesSource_String(t *testing.T) {
	t.Parallel()

	s, err := NewFeatureDoubleValuesSource("features", "pagerank")
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	got := s.String()
	want := "FeatureDoubleValuesSource(features, pagerank)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFeatureDoubleValues_Empty_AlwaysReportsFalse(t *testing.T) {
	t.Parallel()

	v := newEmptyFeatureDoubleValues()

	for _, doc := range []int{0, 1, 5, 1000} {
		ok, err := v.AdvanceExact(doc)
		if err != nil {
			t.Fatalf("AdvanceExact(%d): %v", doc, err)
		}
		if ok {
			t.Errorf("AdvanceExact(%d) on empty reader = true, want false", doc)
		}
	}

	got, err := v.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue: %v", err)
	}
	if got != 0 {
		t.Errorf("DoubleValue on empty reader = %v, want 0", got)
	}
}

func TestFeatureDoubleValues_AdvanceExact_DecodesAcrossDocs(t *testing.T) {
	t.Parallel()

	values := []float32{0.25, 4.0, 17.5}
	docs := []int{0, 3, 7}
	freqs := make([]int, len(values))
	for i, val := range values {
		freqs[i] = int(document.EncodeFeatureValueAsTermFreq(val))
	}

	postings := newFakeFeaturePostings(docs, freqs)
	v := newFeatureDoubleValues(postings)

	cases := []struct {
		doc     int
		wantHit bool
		wantVal float64
	}{
		{doc: 0, wantHit: true, wantVal: float64(values[0])},
		{doc: 1, wantHit: false},
		{doc: 3, wantHit: true, wantVal: float64(values[1])},
		{doc: 5, wantHit: false},
		{doc: 7, wantHit: true, wantVal: float64(values[2])},
	}
	for _, tc := range cases {
		hit, err := v.AdvanceExact(tc.doc)
		if err != nil {
			t.Fatalf("AdvanceExact(%d): %v", tc.doc, err)
		}
		if hit != tc.wantHit {
			t.Errorf("AdvanceExact(%d) = %v, want %v", tc.doc, hit, tc.wantHit)
			continue
		}
		if !hit {
			continue
		}
		got, err := v.DoubleValue()
		if err != nil {
			t.Fatalf("DoubleValue at doc=%d: %v", tc.doc, err)
		}
		if !approxEqualFloat64(got, tc.wantVal, 1e-6) {
			t.Errorf("DoubleValue at doc=%d = %v, want approx %v", tc.doc, got, tc.wantVal)
		}
	}
}

func TestFeatureDoubleValues_AdvanceExact_PastCursorReturnsFalse(t *testing.T) {
	t.Parallel()

	docs := []int{5, 10}
	freqs := []int{
		int(document.EncodeFeatureValueAsTermFreq(1.0)),
		int(document.EncodeFeatureValueAsTermFreq(2.0)),
	}
	postings := newFakeFeaturePostings(docs, freqs)
	v := newFeatureDoubleValues(postings)

	// Pull the cursor forward to docID=10.
	if _, err := v.AdvanceExact(10); err != nil {
		t.Fatalf("AdvanceExact(10): %v", err)
	}
	// Asking for an earlier doc must report miss without retreating.
	hit, err := v.AdvanceExact(5)
	if err != nil {
		t.Fatalf("AdvanceExact(5) after 10: %v", err)
	}
	if hit {
		t.Errorf("AdvanceExact(5) after cursor at 10 = true, want false (no retreat)")
	}
}

func approxEqualFloat64(a, b, tolerance float64) bool {
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
