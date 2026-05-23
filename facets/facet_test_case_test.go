// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// facetTestHelpers provides utility functions shared across facet tests.
// Mirrors org.apache.lucene.facet.FacetTestCase helper methods; converted
// from an abstract Java class to standalone Go helpers in the test binary.

import (
	"math"
	"sort"
	"strings"
	"testing"
)

// sortTies sorts label-value slices that have equal numeric values by label
// (byte order), ensuring deterministic ordering for test assertions.
// Mirrors FacetTestCase.sortTies(LabelAndValue[]).
func sortTies(t *testing.T, labelValues []*LabelAndValue) {
	t.Helper()
	// Stable sort preserving count order; break ties on label.
	sort.SliceStable(labelValues, func(i, j int) bool {
		vi := labelValues[i].Value
		vj := labelValues[j].Value
		if vi != vj {
			return vi > vj // descending count
		}
		return labelValues[i].Label < labelValues[j].Label
	})
}

// sortFacetResultsTies sorts a FacetResult's label values, breaking ties
// on label name. Mirrors FacetTestCase.sortTies(List<FacetResult>).
func sortFacetResultsTies(t *testing.T, results []*FacetResult) {
	t.Helper()
	for _, r := range results {
		sortTies(t, r.LabelValues)
	}
}

// sortLabelValues sorts a slice of LabelAndValue descending by value,
// ties broken by label. Mirrors FacetTestCase.sortLabelValues.
func sortLabelValues(t *testing.T, lvs []*LabelAndValue) {
	t.Helper()
	sort.SliceStable(lvs, func(i, j int) bool {
		vi := lvs[i].Value
		vj := lvs[j].Value
		if vi != vj {
			return vi > vj
		}
		return lvs[i].Label < lvs[j].Label
	})
}

// sortFacetResults sorts results by value desc, ties broken by dim name.
// Mirrors FacetTestCase.sortFacetResults.
func sortFacetResults(t *testing.T, results []*FacetResult) {
	t.Helper()
	sort.SliceStable(results, func(i, j int) bool {
		vi := results[i].Value
		vj := results[j].Value
		if vi != vj {
			return vi > vj
		}
		return results[i].Dim < results[j].Dim
	})
}

// assertFloatValuesEqualsList asserts two sorted []*FacetResult slices contain
// the same float-valued entries within a relative tolerance.
// Mirrors FacetTestCase.assertFloatValuesEquals(List, List).
func assertFloatValuesEqualsList(t *testing.T, a, b []*FacetResult) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("assertFloatValuesEquals: len %d != %d", len(a), len(b))
	}
	aByDim := make(map[string]*FacetResult, len(a))
	for _, r := range a {
		aByDim[r.Dim] = r
	}
	bByDim := make(map[string]*FacetResult, len(b))
	for _, r := range b {
		bByDim[r.Dim] = r
	}
	for dim, ra := range aByDim {
		rb, ok := bByDim[dim]
		if !ok {
			t.Fatalf("assertFloatValuesEquals: dim %q missing in b", dim)
		}
		assertFloatValuesEquals(t, ra, rb)
	}
}

// assertFloatValuesEquals asserts two FacetResult values are equal within a
// relative tolerance. Mirrors FacetTestCase.assertFloatValuesEquals(FacetResult, FacetResult).
func assertFloatValuesEquals(t *testing.T, a, b *FacetResult) {
	t.Helper()
	if a.Dim != b.Dim {
		t.Fatalf("assertFloatValuesEquals: dim %q != %q", a.Dim, b.Dim)
	}
	if !equalStringSlices(a.Path, b.Path) {
		t.Fatalf("assertFloatValuesEquals: path %v != %v", a.Path, b.Path)
	}
	if a.ChildCount != b.ChildCount {
		t.Fatalf("assertFloatValuesEquals: childCount %d != %d", a.ChildCount, b.ChildCount)
	}
	assertNumericValuesNearlyEqual(t, float64(a.Value), float64(b.Value))
	if len(a.LabelValues) != len(b.LabelValues) {
		t.Fatalf("assertFloatValuesEquals: labelValues len %d != %d", len(a.LabelValues), len(b.LabelValues))
	}
	for i := range a.LabelValues {
		if a.LabelValues[i].Label != b.LabelValues[i].Label {
			t.Fatalf("assertFloatValuesEquals[%d]: label %q != %q", i, a.LabelValues[i].Label, b.LabelValues[i].Label)
		}
		assertNumericValuesNearlyEqual(t, float64(a.LabelValues[i].Value), float64(b.LabelValues[i].Value))
	}
}

// assertNumericValuesNearlyEqual asserts two float64 values are within a
// relative tolerance of 1e-5. Mirrors FacetTestCase.assertNumericValuesEquals.
func assertNumericValuesNearlyEqual(t *testing.T, a, b float64) {
	t.Helper()
	if a == b {
		return
	}
	tol := math.Abs(a) / 1e5
	if math.Abs(a-b) > tol {
		t.Fatalf("assertNumericValuesNearlyEqual: |%v - %v| > %v", a, b, tol)
	}
}

// assertFacetResult asserts all fields of a FacetResult match expectations.
// Mirrors FacetTestCase.assertFacetResult.
func assertFacetResult(
	t *testing.T,
	result *FacetResult,
	expectedDim string,
	expectedPath []string,
	expectedChildCount int,
	expectedValue int64,
	expectedChildren ...*LabelAndValue,
) {
	t.Helper()
	if result == nil {
		t.Fatal("assertFacetResult: result is nil")
	}
	if result.Dim != expectedDim {
		t.Errorf("assertFacetResult: dim %q != %q", result.Dim, expectedDim)
	}
	if !equalStringSlices(result.Path, expectedPath) {
		t.Errorf("assertFacetResult: path %v != %v", result.Path, expectedPath)
	}
	if result.ChildCount != expectedChildCount {
		t.Errorf("assertFacetResult: childCount %d != %d", result.ChildCount, expectedChildCount)
	}
	if result.Value != expectedValue {
		t.Errorf("assertFacetResult: value %d != %d", result.Value, expectedValue)
	}
	if len(result.LabelValues) != len(expectedChildren) {
		t.Errorf("assertFacetResult: %d labelValues != %d expected", len(result.LabelValues), len(expectedChildren))
		return
	}
	// order-independent match
	found := make([]bool, len(expectedChildren))
outer:
	for _, lv := range result.LabelValues {
		for i, ec := range expectedChildren {
			if !found[i] && lv.Label == ec.Label && lv.Value == ec.Value {
				found[i] = true
				continue outer
			}
		}
		t.Errorf("assertFacetResult: unexpected labelValue %v", lv)
	}
	for i, f := range found {
		if !f {
			t.Errorf("assertFacetResult: expected labelValue %v not found", expectedChildren[i])
		}
	}
}

// equalStringSlices reports whether two string slices are equal.
func equalStringSlices(a, b []string) bool {
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

// labelAndValue constructs a LabelAndValue for use in assertFacetResult expectations.
func labelAndValue(label string, value int64) *LabelAndValue {
	return &LabelAndValue{Label: label, Value: value}
}

// testDoc mirrors FacetTestCase.TestDoc: a simple document bag used in random tests.
type testDoc struct {
	content string
	dims    []string
	value   float32
}

// getRandomTokens returns count random-ish strings for use in randomised tests.
// Mirrors FacetTestCase.getRandomTokens.
func getRandomTokens(count int) []string {
	tokens := make([]string, count)
	vocab := "abcdefghijklmnopqrstuvwxyz"
	for i := range tokens {
		n := (i%10 + 1) // length 1-10
		var b strings.Builder
		for j := 0; j < n; j++ {
			b.WriteByte(vocab[(i*7+j*13)%len(vocab)])
		}
		tokens[i] = b.String()
	}
	return tokens
}

// pickToken picks a token from the slice with a long-tail distribution.
// Mirrors FacetTestCase.pickToken.
func pickToken(tokens []string, seed int) string {
	for i, tok := range tokens {
		if (seed+i)%2 == 0 {
			return tok
		}
	}
	return tokens[0]
}
