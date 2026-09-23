// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestNorms.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestNormsMaxByteNorms ports testMaxByteNorms, whose buildIndex configures
// MySimProvider, a PerFieldSimilarityWrapper subclass overriding get(String).
// Gocene's PerFieldSimilarityWrapper keeps a per-field map and does not
// dispatch computeNorm(FieldInvertState) through an overridable get(String).
func TestNormsMaxByteNorms(t *testing.T) {
	t.Fatal("org.apache.lucene.search.similarities.PerFieldSimilarityWrapper#get(String) " +
		"(per-field dispatch of computeNorm) is not ported")
}

func TestNormsEmptyValueVsNoValue(t *testing.T) {
	dir := newDirectory()
	cfg := newIndexWriterConfig()
	cfg.SetMergePolicy(newLogMergePolicy())
	w := mustNewIndexWriter(t, dir, cfg)
	doc := document.NewDocument()
	mustAddDocument(t, w, doc)
	doc.Add(newTextField(t, "foo", "", false))
	mustAddDocument(t, w, doc)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	mustClose(t, w)
	leafReader := getOnlyLeafReader(t, reader)
	normValues, err := leafReader.GetNormValues("foo")
	if err != nil {
		t.Fatalf("getNormValues: %v", err)
	}
	if normValues == nil {
		t.Fatal("assertNotNull(normValues)")
	}
	doc0, err := normValues.NextDoc()
	if err != nil || doc0 != 1 { // doc 0 does not have norms
		t.Fatalf("nextDoc: expected 1, got %d (%v)", doc0, err)
	}
	v, err := normValues.LongValue()
	if err != nil || v != 0 {
		t.Fatalf("longValue: expected 0, got %d (%v)", v, err)
	}
	mustClose(t, reader, dir)
}
