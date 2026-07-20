// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// bugReproTokenStream ports the private BugReproTokenStream from
// org.apache.lucene.index.TestSameTokenSamePosition. It emits a fixed sequence
// of four tokens in which "six" and "drunken" each occur twice at the same
// position (the second occurrence carries a zero position increment) and at
// identical start/end offsets. Indexing it once reproduced an April-2011 trunk
// assertion error.
type bugReproTokenStream struct {
	*analysis.BaseTokenStream
	termAtt   analysis.CharTermAttribute
	offsetAtt analysis.OffsetAttribute
	posIncAtt analysis.PositionIncrementAttribute
	terms     []string
	starts    []int
	ends      []int
	incs      []int
	upto      int
}

func newBugReproTokenStream() *bugReproTokenStream {
	base := analysis.NewBaseTokenStream()
	ts := &bugReproTokenStream{
		BaseTokenStream: base,
		termAtt:         analysis.NewCharTermAttribute(),
		offsetAtt:       analysis.NewOffsetAttribute(),
		posIncAtt:       analysis.NewPositionIncrementAttribute(),
		terms:           []string{"six", "six", "drunken", "drunken"},
		starts:          []int{0, 0, 4, 4},
		ends:            []int{3, 3, 11, 11},
		incs:            []int{1, 0, 1, 0},
	}
	base.AddAttribute(ts.termAtt)
	base.AddAttribute(ts.offsetAtt)
	base.AddAttribute(ts.posIncAtt)
	return ts
}

func (b *bugReproTokenStream) IncrementToken() (bool, error) {
	if b.upto == len(b.terms) {
		return false, nil
	}
	b.termAtt.SetEmpty()
	b.termAtt.AppendString(b.terms[b.upto])
	b.offsetAtt.SetOffset(b.starts[b.upto], b.ends[b.upto])
	b.posIncAtt.SetPositionIncrement(b.incs[b.upto])
	b.upto++
	return true, nil
}

func (b *bugReproTokenStream) Reset() error {
	b.upto = 0
	return nil
}

// newSameTokenWriter opens a fresh SimpleFSDirectory-backed IndexWriter.
//
// Divergences from Lucene shared by both tests:
//   - Lucene drives the write through RandomIndexWriter; Gocene exposes no
//     randomized test-writer wrapper, so the plain IndexWriter is used. This
//     test only exercises the indexing path, so the wrapper is irrelevant.
//   - MockAnalyzer is replaced by WhitespaceAnalyzer; the analyzer is unused on
//     a field backed by a pre-built TokenStream.
func newSameTokenWriter(t *testing.T) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	return dir, w
}

// sameTokenSamePositionBlocked gates every test in this file. The upstream
// tests index a TextField constructed directly from a pre-built TokenStream
// (new TextField("eng", new BugReproTokenStream())). Gocene's document.Field
// has no TokenStream-accepting constructor and no TokenStreamValue setter
// (document/field.go, document/field_setters.go), so the field carrying
// bugReproTokenStream cannot be built and the document cannot be indexed.
// This is the same gap that keeps every TestCustomTermFreq port skipped.
//
// The port is kept faithful and complete so it can be unskipped verbatim once
// document.Field gains a TokenStream value type.
const sameTokenSamePositionBlocked = "blocked: document.Field has no TokenStream " +
	"value type (document/field.go, document/field_setters.go); same gap as TestCustomTermFreq"

// sameTokenDoc builds a document with a single "eng" TextField backed by a
// bugReproTokenStream.
func sameTokenDoc(t *testing.T) *document.Document {
	t.Helper()
	field, err := document.NewField("eng", newBugReproTokenStream(), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc := document.NewDocument()
	doc.Add(field)
	return doc
}

// TestSameTokenSamePosition ports
// org.apache.lucene.index.TestSameTokenSamePosition#test: indexing a single
// document whose tokens repeat at the same position must not raise an error.
func TestSameTokenSamePosition(t *testing.T) {
	dir, w := newSameTokenWriter(t)
	defer dir.Close()

	if err := w.AddDocument(sameTokenDoc(t)); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestSameTokenSamePosition_MoreDocs ports
// org.apache.lucene.index.TestSameTokenSamePosition#testMoreDocs: the same
// repeated-position document indexed 100 times must not raise an error.
func TestSameTokenSamePosition_MoreDocs(t *testing.T) {
	dir, w := newSameTokenWriter(t)
	defer dir.Close()

	for i := 0; i < 100; i++ {
		if err := w.AddDocument(sameTokenDoc(t)); err != nil {
			t.Fatalf("AddDocument %d failed: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
