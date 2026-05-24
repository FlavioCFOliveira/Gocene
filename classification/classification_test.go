// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// ClassificationResult
// ---------------------------------------------------------------------------

func TestClassificationResultFields(t *testing.T) {
	ref := util.NewBytesRef([]byte("sport"))
	r := classification.ClassificationResult[*util.BytesRef]{
		AssignedClass: ref,
		Score:         0.9,
	}
	if r.AssignedClass != ref {
		t.Fatalf("expected AssignedClass %v, got %v", ref, r.AssignedClass)
	}
	if r.Score != 0.9 {
		t.Fatalf("expected Score 0.9, got %v", r.Score)
	}
}

func TestClassificationResultCompareHigherScoreWins(t *testing.T) {
	a := classification.ClassificationResult[string]{AssignedClass: "a", Score: 0.8}
	b := classification.ClassificationResult[string]{AssignedClass: "b", Score: 0.3}
	// a has higher score; Compare(b) should return positive (a > b)
	if got := a.Compare(b); got <= 0 {
		t.Fatalf("expected a.Compare(b) > 0, got %d", got)
	}
}

func TestClassificationResultCompareLowerScoreLoses(t *testing.T) {
	a := classification.ClassificationResult[string]{AssignedClass: "a", Score: 0.1}
	b := classification.ClassificationResult[string]{AssignedClass: "b", Score: 0.9}
	if got := a.Compare(b); got >= 0 {
		t.Fatalf("expected a.Compare(b) < 0, got %d", got)
	}
}

func TestClassificationResultCompareEqual(t *testing.T) {
	a := classification.ClassificationResult[string]{AssignedClass: "a", Score: 0.5}
	b := classification.ClassificationResult[string]{AssignedClass: "b", Score: 0.5}
	if got := a.Compare(b); got != 0 {
		t.Fatalf("expected a.Compare(b) == 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Classifier interface compliance (stub implementations return nil)
// ---------------------------------------------------------------------------

func TestSimpleNaiveBayesClassifierReturnsNil(t *testing.T) {
	c := classification.NewSimpleNaiveBayesClassifier(nil, nil, nil, "class", "text")

	r, err := c.AssignClass("hello world")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}

	rs, err := c.GetClasses("hello world")
	if err != nil || rs != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rs, err)
	}

	rsMax, err := c.GetClassesMax("hello world", 3)
	if err != nil || rsMax != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rsMax, err)
	}
}

func TestCachingNaiveBayesClassifierReturnsNil(t *testing.T) {
	c := classification.NewCachingNaiveBayesClassifier(nil, nil, nil, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

func TestBM25NBClassifierReturnsNil(t *testing.T) {
	c := classification.NewBM25NBClassifier(nil, nil, nil, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

func TestKNNClassifierReturnsNil(t *testing.T) {
	c := classification.NewKNearestNeighborClassifier(nil, nil, nil, 5, 1, 1, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

func TestKNNFuzzyClassifierReturnsNil(t *testing.T) {
	c := classification.NewKNearestFuzzyClassifier(nil, nil, nil, 5, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

func TestBooleanPerceptronClassifierDefaultThreshold(t *testing.T) {
	c := classification.NewBooleanPerceptronClassifier(nil, nil, nil, nil, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

func TestBooleanPerceptronClassifierCustomThreshold(t *testing.T) {
	thresh := 0.7
	c := classification.NewBooleanPerceptronClassifier(nil, nil, nil, &thresh, "class", "text")
	r, err := c.AssignClass("hello")
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
}

// ---------------------------------------------------------------------------
// Classifier[T] interface satisfied by concrete types
// ---------------------------------------------------------------------------

func TestClassifierInterfaceSatisfied(t *testing.T) {
	var _ classification.Classifier[*util.BytesRef] = classification.NewSimpleNaiveBayesClassifier(nil, nil, nil, "c")
	var _ classification.Classifier[*util.BytesRef] = classification.NewCachingNaiveBayesClassifier(nil, nil, nil, "c")
	var _ classification.Classifier[*util.BytesRef] = classification.NewBM25NBClassifier(nil, nil, nil, "c")
	var _ classification.Classifier[*util.BytesRef] = classification.NewKNearestNeighborClassifier(nil, nil, nil, 1, 0, 0, "c")
	var _ classification.Classifier[*util.BytesRef] = classification.NewKNearestFuzzyClassifier(nil, nil, nil, 1, "c")
	var _ classification.Classifier[*util.BytesRef] = classification.NewBooleanPerceptronClassifier(nil, nil, nil, nil, "c", "t")
}
