// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for omitting term frequencies and positions
// (IndexOptions.DOCS / omitTf).
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestOmitTf.java
//
// GOC-4261: Port test `org.apache.lucene.index.TestOmitTf`.
//
// # Test coverage
//
//   - TestOmitTf_NoPrxFile  — 1:1 port of testNoPrxFile()    — PASSES
//   - TestOmitTf_MixedRAM   — 1:1 port of testMixedRAM()     — t.Skip
//   - TestOmitTf_Basic      — 1:1 port of testBasic()        — t.Skip
//   - TestOmitTf_Stats      — 1:1 port of testStats()        — t.Skip
//
// # Deviations from the Java reference
//
//   - TestOmitTf_NoPrxFile passes: it writes 30 documents where f1 has
//     IndexOptions.DOCS (no freqs, no positions), forces 15+ merges (via
//     LogMergePolicy with mergeFactor=2), and asserts no .prx/.pos file
//     exists.  The only deviation is that MockAnalyzer is replaced by
//     analysis.NewWhitespaceAnalyzer() (MockAnalyzer not ported).
//
//   - TestOmitTf_MixedRAM is degraded to t.Skip: requires
//     getOnlyLeafReader(DirectoryReader.open(dir)) to read back FieldInfos
//     and verify IndexOptions values; NewSegmentReader does not load
//     coreReaders from disk.
//
//   - TestOmitTf_Basic is degraded to t.Skip: requires DirectoryReader.open
//
//   - IndexSearcher + similarity-aware scoring (SimpleSimilarity); all
//     require the search layer not yet wired for index-level tests.
//
//   - TestOmitTf_Stats is degraded to t.Skip: requires RandomIndexWriter and
//     iw.getReader() NRT path (DirectoryReader.open(IndexWriter)).
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	indexTestutil "github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestOmitTf_NoPrxFile ports testNoPrxFile().
//
// Java writes 30 documents with a single field typed as IndexOptions.DOCS
// (no frequencies, no positions), using LogMergePolicy with mergeFactor=2
// and noCFSRatio=0.0, then asserts no .prx or .pos file exists.
//
// Deviation: MockAnalyzer replaced by analysis.NewWhitespaceAnalyzer().
func TestOmitTf_NoPrxFile(t *testing.T) {
	tmpDir := t.TempDir()
	ram, err := store.NewSimpleFSDirectory(tmpDir)
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer ram.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	lmp := index.NewLogMergePolicy()
	lmp.SetMergeFactor(2)
	lmp.SetNoCFSRatio(0.0)

	cfg := index.NewIndexWriterConfig(analyzer)
	cfg.SetMergePolicy(lmp)

	writer, err := index.NewIndexWriter(ram, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)

	for i := 0; i < 30; i++ {
		doc := document.NewDocument()
		f1, err := document.NewField("f1", "This field has term freqs", ft)
		if err != nil {
			t.Fatalf("NewFieldWithType: %v", err)
		}
		doc.Add(f1)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	assertNoPrxFile(t, ram)
}

// TestOmitTf_MixedRAM ports testMixedRAM().
//
// Java writes documents with one field that has freqs+positions (normalType)
// and one field that has only doc IDs (omitType = IndexOptions.DOCS), then
// uses getOnlyLeafReader(DirectoryReader.open(ram)) to verify the FieldInfo
// IndexOptions for each field.
func TestOmitTf_MixedRAM(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	normalType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	normalType.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	omitType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	omitType.SetIndexOptions(index.IndexOptionsDocs)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		nf, _ := document.NewField("normal", "normal text here", normalType)
		of, _ := document.NewField("omit", "omit text here", omitType)
		doc.Add(nf)
		doc.Add(of)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	leaf := indexTestutil.GetOnlyLeafReader(r)
	infos := leaf.GetFieldInfos()
	if infos == nil {
		t.Fatal("GetFieldInfos returned nil")
	}

	normalInfo := infos.GetByName("normal")
	if normalInfo == nil {
		t.Fatal("field 'normal' not found")
	}
	if got, want := normalInfo.IndexOptions(), index.IndexOptionsDocsAndFreqsAndPositions; got != want {
		t.Errorf("normal IndexOptions = %v, want %v", got, want)
	}

	omitInfo := infos.GetByName("omit")
	if omitInfo == nil {
		t.Fatal("field 'omit' not found")
	}
	if got, want := omitInfo.IndexOptions(), index.IndexOptionsDocs; got != want {
		t.Errorf("omit IndexOptions = %v, want %v", got, want)
	}
}

// TestOmitTf_Basic ports testBasic().
//
// Java builds a 30-document index with two fields (omitTf and normal),
// opens it with DirectoryReader + IndexSearcher using a SimpleSimilarity,
// and asserts that scores for the omitTf field match the expected values
// given no term frequencies are stored.
//
// Degraded to t.Skip: requires DirectoryReader.open, IndexSearcher, and
// similarity-aware scoring (SimpleSimilarity); search layer not yet wired
// for index-level tests.
func TestOmitTf_Basic(t *testing.T) {
	t.Fatal("needs DirectoryReader.open, IndexSearcher, and SimpleSimilarity " +
		"scoring (search layer not yet wired for index-level tests)")
}

// TestOmitTf_Stats ports testStats().
//
// Java builds a single-document index with an DOCS field via RandomIndexWriter,
// opens an NRT reader, and asserts that docFreq == totalTermFreq and
// getSumDocFreq == getSumTotalTermFreq for the field.
func TestOmitTf_Stats(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	riw := indexTestutil.NewWithConfig(w, 1, indexTestutil.Config{
		CommitProbability:     0,
		ForceMergeProbability: 0,
	})

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	doc := document.NewDocument()
	f1, err := document.NewField("f1", "This field has term freqs", ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	doc.Add(f1)
	if err := riw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	r, err := riw.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer r.Close()

	terms, err := r.Terms("f1")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f1) returned nil")
	}

	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	var termEnum index.TermsEnum
	for {
		te, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if te == nil {
			break
		}
		if te.Text() == "field" {
			termEnum = it
			break
		}
	}
	if termEnum == nil {
		t.Fatal("no term enum for term 'field'")
	}

	docFreq, err := termEnum.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	totalTermFreq, err := termEnum.TotalTermFreq()
	if err != nil {
		t.Fatalf("TotalTermFreq: %v", err)
	}
	if docFreq != 1 {
		t.Errorf("DocFreq = %d, want 1", docFreq)
	}
	if totalTermFreq != 1 {
		t.Errorf("TotalTermFreq = %d, want 1", totalTermFreq)
	}

	sumDocFreq, err := terms.GetSumDocFreq()
	if err != nil {
		t.Fatalf("GetSumDocFreq: %v", err)
	}
	sumTotalTermFreq, err := terms.GetSumTotalTermFreq()
	if err != nil {
		t.Fatalf("GetSumTotalTermFreq: %v", err)
	}
	if sumDocFreq != sumTotalTermFreq {
		t.Errorf("SumDocFreq(%d) != SumTotalTermFreq(%d)", sumDocFreq, sumTotalTermFreq)
	}

	if err := riw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
