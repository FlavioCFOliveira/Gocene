// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSameTokenSamePosition.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// bugReproTokenStream ports the package-private final class BugReproTokenStream.
type bugReproTokenStream struct {
	*analysis.BaseTokenStream
	termAtt        analysis.CharTermAttribute
	offsetAtt      analysis.OffsetAttribute
	posIncAtt      tokenattributes.PositionIncrementAttribute
	nextTokenIndex int
}

const bugReproTokenCount = 4

var (
	bugReproTerms  = []string{"six", "six", "drunken", "drunken"}
	bugReproStarts = []int{0, 0, 4, 4}
	bugReproEnds   = []int{3, 3, 11, 11}
	bugReproIncs   = []int{1, 0, 1, 0}
)

func newBugReproTokenStream() *bugReproTokenStream {
	ts := &bugReproTokenStream{BaseTokenStream: analysis.NewBaseTokenStream()}
	ts.termAtt = ts.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ts.offsetAtt = ts.AddAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	ts.posIncAtt = ts.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	return ts
}

func (b *bugReproTokenStream) IncrementToken() (bool, error) {
	if b.nextTokenIndex < bugReproTokenCount {
		b.termAtt.SetValue(bugReproTerms[b.nextTokenIndex])
		b.offsetAtt.SetOffset(bugReproStarts[b.nextTokenIndex], bugReproEnds[b.nextTokenIndex])
		b.posIncAtt.SetPositionIncrement(bugReproIncs[b.nextTokenIndex])
		b.nextTokenIndex++
		return true, nil
	}
	return false, nil
}

func (b *bugReproTokenStream) Reset() error {
	if err := b.BaseTokenStream.Reset(); err != nil {
		return err
	}
	b.nextTokenIndex = 0
	return nil
}

func sameTokenSamePositionDoc(t *testing.T) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewField("eng", newBugReproTokenStream(), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	doc.Add(f)
	return doc
}

// TestSameTokenSamePosition attempts to reproduce an assertion error that
// happens only with the trunk version around April 2011.
func TestSameTokenSamePosition(t *testing.T) {
	dir := newDirectory()
	riw, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	if _, err := riw.AddDocument(sameTokenSamePositionDoc(t)); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	mustClose(t, riw, dir)
}

// TestSameTokenSamePositionMoreDocs is the same as the above, but with more docs.
func TestSameTokenSamePositionMoreDocs(t *testing.T) {
	dir := newDirectory()
	riw, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	for i := 0; i < 100; i++ {
		if _, err := riw.AddDocument(sameTokenSamePositionDoc(t)); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	mustClose(t, riw, dir)
}
