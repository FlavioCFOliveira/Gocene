// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification_test

// Functional tests for the classifier implementations.  Each test builds a
// tiny in-memory index (three "sports" documents and three "politics"
// documents), trains the classifier, and asserts that the most likely class
// for a clearly-sports sentence is "sports" and a clearly-politics sentence
// is "politics".

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	// Register the production codec so postings are flushed correctly.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// ---- shared test infrastructure --------------------------------------------

const (
	classField = "cat"
	textField  = "body"
)

// trainingDocs are (text, class) pairs used to train every classifier.
var trainingDocs = []struct{ text, class string }{
	{"football match goal scored", "sports"},
	{"tennis player wins championship", "sports"},
	{"basketball game overtime victory", "sports"},
	{"election campaign president vote", "politics"},
	{"senate bill congress legislation", "politics"},
	{"government policy parliament debate", "politics"},
}

// buildIndex creates an in-memory index with the training documents and
// returns the opened DirectoryReader plus a cleanup func.
func buildIndex(t *testing.T, stored bool) (index.IndexReaderInterface, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, td := range trainingDocs {
		doc := document.NewDocument()
		// Class field: indexed so Terms() works; stored so Doc() retrieval works.
		sf, err := document.NewStringField(classField, td.class, stored)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)
		// Text field: indexed and tokenized.
		tf, err := document.NewTextField(textField, td.text, false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return r, func() {
		_ = r.Close()
		_ = dir.Close()
	}
}

// assertTopClass verifies that the classifier assigns expectedClass to text.
func assertTopClass(t *testing.T, c classification.Classifier[*util.BytesRef], text, expectedClass string) {
	t.Helper()
	result, err := c.AssignClass(text)
	if err != nil {
		t.Fatalf("AssignClass(%q): %v", text, err)
	}
	if result == nil {
		t.Fatalf("AssignClass(%q): got nil result", text)
	}
	got := result.AssignedClass.String()
	if got != expectedClass {
		t.Errorf("AssignClass(%q) = %q, want %q (score=%.4f)", text, got, expectedClass, result.Score)
	}
}

// ---- SimpleNaiveBayesClassifier functional test ----------------------------

func TestSimpleNaiveBayesClassifier_Functional(t *testing.T) {
	r, cleanup := buildIndex(t, false)
	defer cleanup()

	c := classification.NewSimpleNaiveBayesClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		classField,
		textField,
	)

	assertTopClass(t, c, "goal scored championship", "sports")
	assertTopClass(t, c, "election vote president senate", "politics")
}

func TestSimpleNaiveBayesClassifier_GetClasses(t *testing.T) {
	r, cleanup := buildIndex(t, false)
	defer cleanup()

	c := classification.NewSimpleNaiveBayesClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		classField,
		textField,
	)

	list, err := c.GetClasses("football election")
	if err != nil {
		t.Fatalf("GetClasses: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("GetClasses: expected non-empty list")
	}
	// Scores must sum to approximately 1 (normalised).
	var total float64
	for _, cr := range list {
		total += cr.Score
	}
	if total < 0.99 || total > 1.01 {
		t.Errorf("normalised scores sum = %.4f, want ≈ 1.0", total)
	}
	// List must be in descending score order.
	for i := 1; i < len(list); i++ {
		if list[i].Score > list[i-1].Score {
			t.Errorf("GetClasses result not sorted: [%d].Score=%.4f > [%d].Score=%.4f",
				i, list[i].Score, i-1, list[i-1].Score)
		}
	}
}

func TestSimpleNaiveBayesClassifier_GetClassesMax(t *testing.T) {
	r, cleanup := buildIndex(t, false)
	defer cleanup()

	c := classification.NewSimpleNaiveBayesClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		classField,
		textField,
	)

	list, err := c.GetClassesMax("football championship goal", 1)
	if err != nil {
		t.Fatalf("GetClassesMax: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("GetClassesMax(1) returned %d results, want 1", len(list))
	}
	if list[0].AssignedClass.String() != "sports" {
		t.Errorf("top class = %q, want \"sports\"", list[0].AssignedClass.String())
	}
}

// ---- CachingNaiveBayesClassifier functional test ---------------------------

func TestCachingNaiveBayesClassifier_Functional(t *testing.T) {
	r, cleanup := buildIndex(t, false)
	defer cleanup()

	c := classification.NewCachingNaiveBayesClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		classField,
		textField,
	)

	assertTopClass(t, c, "goal scored championship", "sports")
	assertTopClass(t, c, "election vote president senate", "politics")
}

// ---- BM25NBClassifier functional test --------------------------------------

func TestBM25NBClassifier_Functional(t *testing.T) {
	r, cleanup := buildIndex(t, false)
	defer cleanup()

	c := classification.NewBM25NBClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		classField,
		textField,
	)

	assertTopClass(t, c, "basketball game overtime", "sports")
	assertTopClass(t, c, "senate parliament legislation", "politics")
}

// ---- KNearestNeighborClassifier functional test ----------------------------

func TestKNearestNeighborClassifier_Functional(t *testing.T) {
	// KNN reads stored fields for the class label, so stored=true here.
	r, cleanup := buildIndex(t, true)
	defer cleanup()

	c := classification.NewKNearestNeighborClassifier(
		r,
		analysis.NewWhitespaceAnalyzer(),
		nil,
		3, 1, 1,
		classField,
		textField,
	)

	assertTopClass(t, c, "football goal championship", "sports")
}
