// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/classification/utils"
)

// ---------------------------------------------------------------------------
// DatasetSplitter
// ---------------------------------------------------------------------------

func TestDatasetSplitterConstruction(t *testing.T) {
	s := utils.NewDatasetSplitter(0.1, 0.1)
	if s == nil {
		t.Fatal("expected non-nil DatasetSplitter")
	}
}

func TestDatasetSplitterSplitReturnsNil(t *testing.T) {
	s := utils.NewDatasetSplitter(0.2, 0.1)
	err := s.Split(nil, nil, nil, nil, nil, false, "class")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ConfusionMatrix
// ---------------------------------------------------------------------------

func TestGetConfusionMatrixReturnsNil(t *testing.T) {
	m, err := utils.GetConfusionMatrix(nil, nil, "class", "text", 1000)
	if err != nil || m != nil {
		t.Fatalf("expected nil, nil; got %v, %v", m, err)
	}
}

// ---------------------------------------------------------------------------
// DocToDoubleVectorUtils
// ---------------------------------------------------------------------------

func TestToSparseLocalFreqDoubleArrayReturnsNil(t *testing.T) {
	v, err := utils.ToSparseLocalFreqDoubleArray(nil, nil)
	if err != nil || v != nil {
		t.Fatalf("expected nil, nil; got %v, %v", v, err)
	}
}

func TestToDenseLocalFreqDoubleArrayReturnsNil(t *testing.T) {
	v, err := utils.ToDenseLocalFreqDoubleArray(nil)
	if err != nil || v != nil {
		t.Fatalf("expected nil, nil; got %v, %v", v, err)
	}
}

// ---------------------------------------------------------------------------
// NearestFuzzyQuery
// ---------------------------------------------------------------------------

func TestNearestFuzzyQueryConstruction(t *testing.T) {
	q := utils.NewNearestFuzzyQuery(nil)
	if q == nil {
		t.Fatal("expected non-nil NearestFuzzyQuery")
	}
}

func TestNearestFuzzyQueryAddTerm(t *testing.T) {
	q := utils.NewNearestFuzzyQuery(nil)
	// should not panic
	q.AddTermToQuery("title", 2, "lucene")
}

func TestNearestFuzzyQueryString(t *testing.T) {
	q := utils.NewNearestFuzzyQuery(nil)
	if q.String() == "" {
		t.Fatal("expected non-empty String()")
	}
}

func TestNearestFuzzyQueryEquals(t *testing.T) {
	q1 := utils.NewNearestFuzzyQuery(nil)
	q1.AddTermToQuery("title", 2, "lucene")

	q2 := utils.NewNearestFuzzyQuery(nil)
	q2.AddTermToQuery("title", 2, "lucene")

	if !q1.Equals(q2) {
		t.Fatal("expected q1.Equals(q2) to be true")
	}
}

func TestNearestFuzzyQueryNotEquals(t *testing.T) {
	q1 := utils.NewNearestFuzzyQuery(nil)
	q1.AddTermToQuery("title", 2, "lucene")

	q2 := utils.NewNearestFuzzyQuery(nil)
	q2.AddTermToQuery("title", 1, "lucene")

	if q1.Equals(q2) {
		t.Fatal("expected q1.Equals(q2) to be false")
	}
}

func TestNearestFuzzyQueryHashCode(t *testing.T) {
	q1 := utils.NewNearestFuzzyQuery(nil)
	q1.AddTermToQuery("title", 2, "lucene")

	q2 := utils.NewNearestFuzzyQuery(nil)
	q2.AddTermToQuery("title", 2, "lucene")

	if q1.HashCode() != q2.HashCode() {
		t.Fatal("expected equal HashCodes for equal queries")
	}
}
