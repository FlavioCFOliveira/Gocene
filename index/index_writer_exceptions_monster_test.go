// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test ports of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterExceptions.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs them only when nightly tests are enabled.

package index_test

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// tooManyTokensStream renders the anonymous TokenStream of testTooManyTokens:
// Integer.MAX_VALUE + 1 tokens "a", all at the same position.
type tooManyTokensStream struct {
	*analysis.BaseTokenStream
	termAtt   analysis.CharTermAttribute
	posIncAtt tokenattributes.PositionIncrementAttribute
	num       int64
}

func newTooManyTokensStream() *tooManyTokensStream {
	s := &tooManyTokensStream{BaseTokenStream: analysis.NewBaseTokenStream()}
	s.termAtt = s.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	s.posIncAtt = s.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	return s
}

func (s *tooManyTokensStream) IncrementToken() (bool, error) {
	if s.num == math.MaxInt32+1 {
		return false, nil
	}
	s.ClearAttributes()
	if s.num == 0 {
		s.posIncAtt.SetPositionIncrement(1)
	} else {
		s.posIncAtt.SetPositionIncrement(0)
	}
	s.termAtt.AppendString("a")
	s.num++
	return true, nil
}

// kind of slow, but omits positions, so just CPU
func TestIndexWriterExceptionsTooManyTokens(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(nil))
	doc := document.NewDocument()
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	f, err := document.NewField("foo", newTooManyTokensStream(), ft)
	if err != nil {
		t.Fatalf("new Field: %v", err)
	}
	doc.Add(f)

	_, err = iw.AddDocument(doc)
	if err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
	if !strings.Contains(err.Error(), "too many tokens") {
		t.Fatalf("expected.getMessage() does not contain \"too many tokens\": %v", err)
	}

	mustClose(t, iw, dir)
}

// TODO: can be super slow in pathological cases (merge config?)
func TestIndexWriterExceptionsMergeExceptionIsTragic(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The Failure's eval calls callStackContainsAnyOf("merge").
	t.Fatal(callStackContainsMissing)
}
