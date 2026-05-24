// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/classification/document"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestDocumentClassifierInterfaceSatisfied verifies that both concrete types
// implement DocumentClassifier at compile time (the interface assertion is in
// document_classifier.go; this test confirms the package builds correctly).
func TestDocumentClassifierInterfaceSatisfied(t *testing.T) {
	var _ document.DocumentClassifier[*util.BytesRef] = document.NewKNearestNeighborDocumentClassifier(
		nil, nil, nil, 5, 1, 1, "class", nil, "text",
	)
	var _ document.DocumentClassifier[*util.BytesRef] = document.NewSimpleNaiveBayesDocumentClassifier(
		nil, nil, "class", nil, "text",
	)
}

func TestKNNDocumentClassifierReturnsNil(t *testing.T) {
	c := document.NewKNearestNeighborDocumentClassifier(nil, nil, nil, 3, 1, 1, "class", nil, "text")
	r, err := c.AssignClass(nil)
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
	rs, err := c.GetClasses(nil)
	if err != nil || rs != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rs, err)
	}
	rsMax, err := c.GetClassesMax(nil, 2)
	if err != nil || rsMax != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rsMax, err)
	}
}

func TestSimpleNaiveBayesDocumentClassifierReturnsNil(t *testing.T) {
	c := document.NewSimpleNaiveBayesDocumentClassifier(nil, nil, "class", nil, "text")
	r, err := c.AssignClass(nil)
	if err != nil || r != nil {
		t.Fatalf("expected nil, nil; got %v, %v", r, err)
	}
	rs, err := c.GetClasses(nil)
	if err != nil || rs != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rs, err)
	}
	rsMax, err := c.GetClassesMax(nil, 2)
	if err != nil || rsMax != nil {
		t.Fatalf("expected nil, nil; got %v, %v", rsMax, err)
	}
}
