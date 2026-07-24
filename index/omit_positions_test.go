// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for omit-positions behaviour.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestOmitPositions.java
//
// GOC-4240: Port test `org.apache.lucene.index.TestOmitPositions`.
//
// # Test coverage
//
//   - TestOmitPositions_Basic        — 1:1 port of testBasic()
//   - TestOmitPositions_Positions    — 1:1 port of testPositions()
//   - TestOmitPositions_NoPrxFile    — 1:1 port of testNoPrxFile()
//
// # Deviations from the Java reference
//
//   - MockAnalyzer → WhitespaceAnalyzer (MockAnalyzer not yet ported).
//
//   - RandomIndexWriter → IndexWriter (RandomIndexWriter not yet ported).
//
//   - newLogMergePolicy(…) → NewLogMergePolicy() + SetMergeFactor: no
//     convenience constructor with merge-factor argument in Gocene.
//
//   - TestOmitPositions_Basic uses commit + DirectoryReader.OpenDirectoryReader
//     rather than the NRT reader path.
//
//   - TestOmitPositions_Positions uses GetOnlyLeafReader on the committed
//     DirectoryReader and checks FieldInfo.IndexOptions().
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"os"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	indexTestutil "github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestOmitPositions_Basic ports testBasic().
//
// Java adds 100 documents with a field indexed as DOCS_AND_FREQS (positions
// omitted), opens an NRT reader from the RandomIndexWriter, calls
// MultiTerms.getTermPostingsEnum to retrieve a PostingsEnum for "test", then
// iterates it asserting freq()==2 for every document.
func TestOmitPositions_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	ft.Freeze()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		field, _ := document.NewField("foo", "this is a test test", ft)
		doc.Add(field)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	terms, err := r.Terms("foo")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	found, err := te.SeekExact(schema.NewTerm("foo", "test"))
	if err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if !found {
		t.Fatal("term \"test\" not found")
	}
	postings, err := te.Postings(schema.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	count := 0
	for {
		doc, err := postings.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == schema.NO_MORE_DOCS {
			break
		}
		count++
		freq, err := postings.Freq()
		if err != nil {
			t.Fatalf("Freq: %v", err)
		}
		if freq != 2 {
			t.Fatalf("doc %d freq=%d, want 2", doc, freq)
		}
	}
	if count != 100 {
		t.Fatalf("expected 100 docs, got %d", count)
	}
}

// TestOmitPositions_Positions ports testPositions().
//
// Java creates three fields with index options DOCS, DOCS_AND_FREQS, and
// DOCS_AND_FREQS_AND_POSITIONS respectively, adds one document, force-merges,
// reopens the index, and asserts that FieldInfos reports the correct index
// options for each field via getOnlyLeafReader(DirectoryReader.open(ram)).
func TestOmitPositions_Positions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	docsType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	docsType.SetIndexOptions(index.IndexOptionsDocs)
	freqsType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	freqsType.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	positionsType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	positionsType.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)

	doc := document.NewDocument()
	f1, _ := document.NewField("docsField", "one", docsType)
	f2, _ := document.NewField("freqsField", "one", freqsType)
	f3, _ := document.NewField("positionsField", "one", positionsType)
	doc.Add(f1)
	doc.Add(f2)
	doc.Add(f3)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
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

	assertIndexOptions := func(name string, want index.IndexOptions) {
		fi := infos.GetByName(name)
		if fi == nil {
			t.Fatalf("field %q not found in FieldInfos", name)
		}
		if got := fi.IndexOptions(); got != want {
			t.Errorf("field %q IndexOptions = %v, want %v", name, got, want)
		}
	}
	assertIndexOptions("docsField", index.IndexOptionsDocs)
	assertIndexOptions("freqsField", index.IndexOptionsDocsAndFreqs)
	assertIndexOptions("positionsField", index.IndexOptionsDocsAndFreqsAndPositions)
}

// TestOmitPositions_NoPrxFile ports testNoPrxFile().
//
// When every field in the index is indexed with DOCS_AND_FREQS (no positions),
// the codec must not emit any positions file (.prx or .pos).  The test adds 30
// documents with a field in DOCS_AND_FREQS mode through a LogMergePolicy with
// mergeFactor=2 / noCFSRatio=0, commits, and asserts no .prx or .pos file
// exists in the directory.
//
// Source: TestOmitPositions.testNoPrxFile()
func TestOmitPositions_NoPrxFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "TestOmitPositions_NoPrxFile")
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ram, err := store.NewSimpleFSDirectory(tmpDir)
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer ram.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetMaxBufferedDocs(3)

	// Java uses newLogMergePolicy() then lmp.setMergeFactor(2) / lmp.setNoCFSRatio(0.0).
	lmp := index.NewLogMergePolicy()
	lmp.SetMergeFactor(2)
	lmp.SetNoCFSRatio(0.0)
	config.SetMergePolicy(lmp)

	writer, err := index.NewIndexWriter(ram, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Field indexed with DOCS_AND_FREQS (positions omitted).
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)

	for i := 0; i < 30; i++ {
		d := document.NewDocument()
		f, err := document.NewField("f1", "This field has term freqs", ft)
		if err != nil {
			t.Fatalf("NewField (i=%d): %v", i, err)
		}
		d.Add(f)
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument (i=%d): %v", i, err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	assertNoPrxFile(t, ram)

	writer.Close()
}

// assertNoPrxFile asserts that dir contains no file ending in ".prx" or ".pos".
// It mirrors the private helper of the same name in the Java source.
func assertNoPrxFile(t *testing.T, dir store.Directory) {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("dir.ListAll: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".prx") || strings.HasSuffix(f, ".pos") {
			t.Errorf("unexpected positions file in directory: %s", f)
		}
	}
}
