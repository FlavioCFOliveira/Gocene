// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestTerms.java
// (Apache Lucene 10.5.0). The Java class uses the test-framework
// RandomIndexWriter (tests/index imports index), so the port lives in the
// external index_test package.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

func TestTermsTermMinMaxBasic(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	w, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, err := document.NewTextField("field", "a b c cc ddd", false)
	if err != nil {
		t.Fatalf("newTextField: %v", err)
	}
	doc.Add(f)
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	terms, err := index.MultiTermsGetTerms(r, "field")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	if terms == nil {
		t.Fatal("MultiTerms.getTerms returned null")
	}
	minTerm, err := terms.GetMin()
	if err != nil {
		t.Fatalf("getMin: %v", err)
	}
	if minTerm == nil || string(minTerm.Bytes.ValidBytes()) != "a" {
		t.Fatalf("getMin: expected \"a\", got %v", minTerm)
	}
	maxTerm, err := terms.GetMax()
	if err != nil {
		t.Fatalf("getMax: %v", err)
	}
	if maxTerm == nil || string(maxTerm.Bytes.ValidBytes()) != "ddd" {
		t.Fatalf("getMax: expected \"ddd\", got %v", maxTerm)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

// TestTermsTermMinMaxRandom ports testTermMinMaxRandom, which indexes
// CannedBinaryTokenStream token streams of random binary terms.
func TestTermsTermMinMaxRandom(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.analysis.CannedBinaryTokenStream is not ported: " +
		"TestTerms.testTermMinMaxRandom indexes CannedBinaryTokenStream.BinaryToken streams")
}
