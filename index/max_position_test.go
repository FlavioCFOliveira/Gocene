// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// maxPositionAnalyzer is a test-only analyzer that ignores the supplied
// reader and always returns the same canned token sequence. It is used to
// inject tokens with explicit position increments into the indexing chain.
type maxPositionAnalyzer struct {
	factory func() analysis.TokenStream
}

func (a *maxPositionAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.factory(), nil
}

func (a *maxPositionAnalyzer) Close() error { return nil }

// newMaxPositionWriter creates an IndexWriter over a RAMDirectory using the
// supplied canned-token analyzer and a simple IndexWriterConfig.
func newMaxPositionWriter(t *testing.T, a analysis.Analyzer) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(a)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return dir, writer
}

// addMaxPositionDoc indexes one document whose "foo" field emits two "foo"
// tokens via the supplied canned-token analyzer factory.
func addMaxPositionDoc(t *testing.T, writer *index.IndexWriter, factory func() analysis.TokenStream) error {
	t.Helper()
	doc := document.NewDocument()
	field, err := document.NewTextField("foo", "ignored", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(field)
	return writer.AddDocument(doc)
}

// TestMaxPosition_TooBigPosition ports TestMaxPosition.testTooBigPosition.
//
// Upstream indexes one document whose "foo" TextField is a CannedTokenStream
// of two "foo" tokens: t1 at position 1 (positionIncrement 2) and t2 with
// positionIncrement == MAX_POSITION, which overflows the maximum. addDocument
// must throw IllegalArgumentException, and a reader opened on the writer must
// then report numDocs() == 0 (the document is not visible).
func TestMaxPosition_TooBigPosition(t *testing.T) {
	dir, writer := newMaxPositionWriter(t, &maxPositionAnalyzer{
		factory: func() analysis.TokenStream {
			return testutil.NewCannedTokenStream(
				testutil.NewTokenWithPosInc("foo", 2, 0, 3),
				testutil.NewTokenWithPosInc("foo", index.MaxPosition, 0, 3),
			)
		},
	})
	defer writer.Close()

	err := addMaxPositionDoc(t, writer, nil)
	if err == nil {
		t.Fatalf("AddDocument with position > MaxPosition should fail")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %v", err)
	}

	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 0 {
		t.Fatalf("NumDocs = %d, want 0", got)
	}

	dir.Close()
}

// TestMaxPosition_MaxPosition ports TestMaxPosition.testMaxPosition.
//
// Upstream indexes one document whose "foo" TextField is a CannedTokenStream
// of two "foo" tokens: t1 at position 0 and t2 with positionIncrement ==
// MAX_POSITION, landing exactly on the maximum. addDocument must succeed; a
// reader opened on the writer must report numDocs() == 1, and the PostingsEnum
// for term "foo" must report freq()==2 with positions 0 and MAX_POSITION.
func TestMaxPosition_MaxPosition(t *testing.T) {
	dir, writer := newMaxPositionWriter(t, &maxPositionAnalyzer{
		factory: func() analysis.TokenStream {
			return testutil.NewCannedTokenStream(
				testutil.NewTokenWithPosInc("foo", 1, 0, 3),
				testutil.NewTokenWithPosInc("foo", index.MaxPosition, 0, 3),
			)
		},
	})
	defer writer.Close()

	if err := addMaxPositionDoc(t, writer, nil); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	reader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 1 {
		t.Fatalf("NumDocs = %d, want 1", got)
	}

	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0]
	terms, err := leaf.Terms("foo")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	postings, err := terms.GetPostingsReader("foo", schema.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("GetPostingsReader: %v", err)
	}
	if postings == nil {
		t.Fatalf("postings for 'foo' is nil")
	}
	docID, err := postings.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID == schema.NO_MORE_DOCS {
		t.Fatalf("no docs for term 'foo'")
	}
	freq, err := postings.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	if freq != 2 {
		t.Fatalf("freq = %d, want 2", freq)
	}
	positions := make([]int, 0, freq)
	for i := 0; i < freq; i++ {
		pos, err := postings.NextPosition()
		if err != nil {
			t.Fatalf("NextPosition: %v", err)
		}
		positions = append(positions, pos)
	}
	if positions[0] != 0 || positions[1] != index.MaxPosition {
		t.Fatalf("positions = %v, want [0 %d]", positions, index.MaxPosition)
	}

	dir.Close()
}
