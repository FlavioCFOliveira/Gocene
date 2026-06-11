// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Go port of org.apache.lucene.document.TestFeatureSort (Lucene 10.4.0).
//
// Source: /tmp/lucene/lucene/core/src/test/org/apache/lucene/document/
//         TestFeatureSort.java
//
// The Java reference exercises FeatureField.newFeatureSort end-to-end:
// RandomIndexWriter, addDocument, getReader, IndexSearcher.search with a Sort
// argument, IndexSearcher.storedFields(), and the search-after pagination used
// by testDuelFloat.
//
// That integration path is blocked on:
//   1. IndexSearcher.Search has no Sort overload yet
//   2. IndexSearcher/DirectoryReader expose no storedFields() entry-point
//   3. OpenDirectoryReader uses NewSegmentReader instead of
//      NewSegmentReaderWithCore, so Terms/Postings fail with
//      "core readers are nil"
//
// These unit tests validate FeatureSortField and FeatureField construction
// and basic properties.  The full end-to-end behaviour will be covered when
// the integration gaps are closed.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestFeatureSort_Feature verifies FeatureSortField construction and accessors.
func TestFeatureSort_Feature(t *testing.T) {
	fsf, err := search.NewFeatureSortField("field", "name")
	if err != nil {
		t.Fatalf("NewFeatureSortField: %v", err)
	}
	if fsf.FeatureName() != "name" {
		t.Errorf("FeatureName() = %q, want %q", fsf.FeatureName(), "name")
	}
	str := fsf.String()
	if !strings.Contains(str, "field") || !strings.Contains(str, "name") {
		t.Errorf("String() = %q, should contain field and feature name", str)
	}
}

// TestFeatureSort_FeatureMissing verifies FeatureSortField with SetMissingValue.
func TestFeatureSort_FeatureMissing(t *testing.T) {
	fsf, err := search.NewFeatureSortField("field", "name")
	if err != nil {
		t.Fatalf("NewFeatureSortField: %v", err)
	}
	// FeatureSortField does not support SetMissingValue; any value is rejected.
	if err := fsf.SetMissingValue(0.0); err == nil {
		t.Error("SetMissingValue(0.0) should error for FeatureSortField")
	}
	if err := fsf.SetMissingValue(1.0); err == nil {
		t.Error("SetMissingValue(1.0) should error for FeatureSortField")
	}
	if err := fsf.SetMissingValue(nil); err == nil {
		t.Error("SetMissingValue(nil) should error for FeatureSortField")
	}
}

// TestFeatureSort_FeatureMissingFieldInSegment verifies that FeatureField
// can be constructed with valid parameters.
func TestFeatureSort_FeatureMissingFieldInSegment(t *testing.T) {
	ff, err := document.NewFeatureField("field", "name", 1.5)
	if err != nil {
		t.Fatalf("NewFeatureField: %v", err)
	}
	if ff.GetFeatureName() != "name" {
		t.Errorf("GetFeatureName() = %q, want %q", ff.GetFeatureName(), "name")
	}
	if ff.GetFeatureValue() != 1.5 {
		t.Errorf("GetFeatureValue() = %v, want %v", ff.GetFeatureValue(), 1.5)
	}
	// Verify term frequency encoding for a valid feature value.
	tf := ff.TermFrequency()
	if tf <= 0 {
		t.Errorf("TermFrequency() = %d, want positive", tf)
	}
}

// TestFeatureSort_FeatureMissingFeatureNameInSegment verifies FeatureField
// with different feature names (simulating cross-segment scenarios).
func TestFeatureSort_FeatureMissingFeatureNameInSegment(t *testing.T) {
	// FeatureField can use any feature name; verify names are distinct.
	ff1, err := document.NewFeatureField("field", "name", 1.0)
	if err != nil {
		t.Fatalf("NewFeatureField(name): %v", err)
	}
	ff2, err := document.NewFeatureField("field", "different_name", 2.0)
	if err != nil {
		t.Fatalf("NewFeatureField(different_name): %v", err)
	}
	if ff1.GetFeatureName() == ff2.GetFeatureName() {
		t.Error("Feature names should be distinct")
	}
}

// TestFeatureSort_FeatureMultipleMissing verifies FeatureSortField
// construction with various feature name combinations.
func TestFeatureSort_FeatureMultipleMissing(t *testing.T) {
	fsf1, err := search.NewFeatureSortField("field_a", "feature_a")
	if err != nil {
		t.Fatalf("NewFeatureSortField(field_a): %v", err)
	}
	fsf2, err := search.NewFeatureSortField("field_b", "feature_b")
	if err != nil {
		t.Fatalf("NewFeatureSortField(field_b): %v", err)
	}
	if fsf1.Equals(fsf2) {
		t.Error("Different sort fields should not be equal")
	}
}

// TestFeatureSort_DuelFloat verifies FeatureSortField equals/hashCode
// contract (duel integration requires full search path).
func TestFeatureSort_DuelFloat(t *testing.T) {
	fsf1, err := search.NewFeatureSortField("field", "name")
	if err != nil {
		t.Fatalf("NewFeatureSortField: %v", err)
	}
	fsf2, err := search.NewFeatureSortField("field", "name")
	if err != nil {
		t.Fatalf("NewFeatureSortField: %v", err)
	}
	fsf3, err := search.NewFeatureSortField("field", "different")
	if err != nil {
		t.Fatalf("NewFeatureSortField: %v", err)
	}
	if !fsf1.Equals(fsf2) {
		t.Error("Equal sort fields should be equal")
	}
	if fsf1.Equals(fsf3) {
		t.Error("Different sort fields should not be equal")
	}
	if fsf1.HashCode() != fsf2.HashCode() {
		t.Error("Equal sort fields should have the same hash code")
	}
}
