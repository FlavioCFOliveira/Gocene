// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.SandboxFacetTestCase.
//
// SandboxFacetTestCase is an abstract test base class in Java. In Go it is
// represented as a collection of test helper functions used by the concrete
// facet test files in this package.
//
// Deviations from Java:
//   - The taxonomy-based helpers (getTopChildrenByCount, getAllChildren,
//     getAllSortByOrd, getSpecificValue, getCountsForRecordedCandidates,
//     getNewSearcherForDrillSideways) require TaxonomyReader and IndexSearcher;
//     deferred to backlog #2693.
//   - assertNumericValuesEquals float tolerance is ported directly; integer
//     equality is exact as in Java.
package facet

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// valueCantBeComputed mirrors SandboxFacetTestCase.VALUE_CANT_BE_COMPUTED.
// Indicates that the overall facet value cannot be derived from the recorder.
const valueCantBeComputed int64 = math.MinInt32

// assertNumericValuesEquals mirrors
// SandboxFacetTestCase.assertNumericValuesEquals with float tolerance.
// For int64 values equality is exact.
func assertNumericValuesEquals(t *testing.T, a, b int64) {
	t.Helper()
	if a != b {
		t.Errorf("numeric values differ: %d != %d", a, b)
	}
}

// assertNumericFloat32Equals mirrors the float branch of
// SandboxFacetTestCase.assertNumericValuesEquals.
func assertNumericFloat32Equals(t *testing.T, a, b float32) {
	t.Helper()
	if a == 0 {
		if b != 0 {
			t.Errorf("float32 values differ: %v != %v", a, b)
		}
		return
	}
	tol := float64(a) / 1e5
	if math.Abs(float64(a)-float64(b)) > math.Abs(tol) {
		t.Errorf("float32 values differ beyond tolerance: %v != %v (tol %v)", a, b, tol)
	}
}

// assertNumericFloat64Equals mirrors the double branch of
// SandboxFacetTestCase.assertNumericValuesEquals.
func assertNumericFloat64Equals(t *testing.T, a, b float64) {
	t.Helper()
	if a == 0 {
		if b != 0 {
			t.Errorf("float64 values differ: %v != %v", a, b)
		}
		return
	}
	tol := a / 1e5
	if math.Abs(a-b) > math.Abs(tol) {
		t.Errorf("float64 values differ beyond tolerance: %v != %v (tol %v)", a, b, tol)
	}
}

// assertFacetResult mirrors SandboxFacetTestCase.assertFacetResult.
// It verifies dim, path, childCount, overall value, and that the expected
// label/value pairs are all present (in any order).
func assertFacetResult(
	t *testing.T,
	result *facets.FacetResult,
	expectedDim string,
	expectedPath []string,
	expectedChildCount int,
	expectedValue int64,
	expectedChildren ...*facets.LabelAndValue,
) {
	t.Helper()
	if result.Dim != expectedDim {
		t.Errorf("Dim = %q; want %q", result.Dim, expectedDim)
	}
	if len(result.Path) != len(expectedPath) {
		t.Errorf("Path = %v; want %v", result.Path, expectedPath)
	} else {
		for i, p := range expectedPath {
			if result.Path[i] != p {
				t.Errorf("Path[%d] = %q; want %q", i, result.Path[i], p)
			}
		}
	}
	if result.ChildCount != expectedChildCount {
		t.Errorf("ChildCount = %d; want %d", result.ChildCount, expectedChildCount)
	}
	assertNumericValuesEquals(t, result.Value, expectedValue)
	if len(result.LabelValues) != len(expectedChildren) {
		t.Errorf("LabelValues len = %d; want %d: %v", len(result.LabelValues), len(expectedChildren), result.LabelValues)
		return
	}
	// Order-independent membership check.
	for _, want := range expectedChildren {
		found := false
		for _, got := range result.LabelValues {
			if got.Label == want.Label && got.Value == want.Value {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected child %v not found in %v", want, result.LabelValues)
		}
	}
}

// ---- self-tests for the helper functions ----

// TestSandboxFacetTestCase_AssertFacetResult verifies assertFacetResult with a
// known FacetResult.
func TestSandboxFacetTestCase_AssertFacetResult(t *testing.T) {
	fr := &facets.FacetResult{
		Dim:        "color",
		Path:       []string{},
		Value:      10,
		ChildCount: 2,
		LabelValues: []*facets.LabelAndValue{
			{Label: "red", Value: 6},
			{Label: "blue", Value: 4},
		},
	}
	assertFacetResult(t, fr, "color", []string{}, 2, 10,
		&facets.LabelAndValue{Label: "red", Value: 6},
		&facets.LabelAndValue{Label: "blue", Value: 4},
	)
}

// TestSandboxFacetTestCase_AssertNumericValuesEquals verifies exact and
// approximate equality helpers.
func TestSandboxFacetTestCase_AssertNumericValuesEquals(t *testing.T) {
	assertNumericValuesEquals(t, 42, 42)
	assertNumericFloat32Equals(t, 1.0, 1.0)
	assertNumericFloat64Equals(t, 3.14, 3.14)
}

// TestSandboxFacetTestCase_ValueCantBeComputed verifies the sentinel constant.
func TestSandboxFacetTestCase_ValueCantBeComputed(t *testing.T) {
	if valueCantBeComputed >= 0 {
		t.Errorf("valueCantBeComputed = %d; want negative sentinel", valueCantBeComputed)
	}
}
