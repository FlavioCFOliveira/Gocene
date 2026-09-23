// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestPrefixRandom.java
// (Apache Lucene 10.5.0).
//
// Create an index with random unicode terms Generates random prefix queries,
// and validates against a simple impl.

package search_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// prefixRandomTestCase renders the fields of TestPrefixRandom.
type prefixRandomTestCase struct {
	searcher *search.IndexSearcher
	reader   *index.DirectoryReader
}

// prSetUp renders setUp(); the returned function renders tearDown().
func prSetUp(t *testing.T) (*prefixRandomTestCase, func()) {
	t.Helper()
	tc := &prefixRandomTestCase{}
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)

	doc := document.NewDocument()
	field := newStringField(t, "field", "", false)
	doc.Add(field)

	num := atLeast(1000)
	for i := 0; i < num; i++ {
		field.SetStringValue(randomUnicodeStringWithMaxLength(random(), 10))
		mustAddDocument(t, writer, doc)
	}
	tc.reader = mustGetReader(t, writer)
	tearDown := func() {
		mustClose(t, tc.reader, dir)
	}
	tc.searcher = newSearcher(t, tc.reader)
	mustClose(t, writer)
	return tc, tearDown
}

// dumbPrefixQuery renders the private static class DumbPrefixQuery: a stupid
// prefix query that just blasts thru the terms.
type dumbPrefixQuery struct {
	*search.MultiTermQuery
	prefix *util.BytesRef
}

// newDumbPrefixQuery renders DumbPrefixQuery(Term).
func newDumbPrefixQuery(term *index.Term) *dumbPrefixQuery {
	q := &dumbPrefixQuery{
		MultiTermQuery: search.NewMultiTermQuery(term.Field, search.ConstantScoreBlendedRewrite),
		prefix:         term.BytesValue(),
	}
	q.MultiTermQuery.SetOwner(q)
	return q
}

// GetTermsEnumWithAttributes renders getTermsEnum(Terms, AttributeSource).
func (q *dumbPrefixQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	it, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	return newSimplePrefixTermsEnum(it, q.prefix), nil
}

// simplePrefixTermsEnum renders DumbPrefixQuery.SimplePrefixTermsEnum.
type simplePrefixTermsEnum struct {
	*index.FilteredTermsEnum
	prefix *util.BytesRef
}

func newSimplePrefixTermsEnum(tenum index.TermsEnum, prefix *util.BytesRef) *simplePrefixTermsEnum {
	e := &simplePrefixTermsEnum{prefix: prefix}
	e.FilteredTermsEnum = index.NewFilteredTermsEnum(tenum, e)
	e.SetInitialSeekTerm(index.NewTermFromBytes("", []byte("")))
	return e
}

// Accept renders accept(BytesRef).
func (e *simplePrefixTermsEnum) Accept(term *spi.Term) (index.AcceptStatus, error) {
	if util.StartsWith(term.BytesValue(), e.prefix) {
		return index.AcceptYes, nil
	}
	return index.AcceptNo, nil
}

// NextSeekTerm keeps the inherited FilteredTermsEnum.nextSeekTerm (the
// initial seek term).
func (e *simplePrefixTermsEnum) NextSeekTerm(current *spi.Term) (*spi.Term, error) {
	return nil, nil
}

// ToString renders toString(String field).
func (q *dumbPrefixQuery) ToString(field string) string {
	return field + ":" + javaBytesRefToString(q.prefix)
}

func (q *dumbPrefixQuery) String() string { return q.ToString("") }

// Visit renders visit(QueryVisitor), which is empty.
func (q *dumbPrefixQuery) Visit(visitor search.QueryVisitor) {}

// Equals renders equals(Object).
func (q *dumbPrefixQuery) Equals(obj spi.Query) bool {
	if !q.MultiTermQuery.Equals(obj) {
		return false
	}
	that := obj.(*dumbPrefixQuery)
	return util.BytesRefEquals(q.prefix, that.prefix)
}

// javaBytesRefToString renders BytesRef.toString(): the bytes as lower-case
// hex, space separated, in brackets.
func javaBytesRefToString(b *util.BytesRef) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range b.ValidBytes() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatInt(int64(v), 16))
	}
	sb.WriteByte(']')
	return sb.String()
}

// test a bunch of random prefixes
func TestPrefixRandomPrefixes(t *testing.T) {
	tc, tearDown := prSetUp(t)
	defer tearDown()
	num := atLeast(100)
	for i := 0; i < num; i++ {
		tc.assertSame(t, randomUnicodeStringWithMaxLength(random(), 5))
	}
}

// assertSame checks that the # of hits is the same as from a very simple
// prefixquery implementation.
func (tc *prefixRandomTestCase) assertSame(t *testing.T, prefix string) {
	t.Helper()
	smart := search.NewPrefixQuery(index.NewTerm("field", prefix))
	dumb := newDumbPrefixQuery(index.NewTerm("field", prefix))

	smartDocs := mustSearch(t, tc.searcher, smart, 25)
	dumbDocs := mustSearch(t, tc.searcher, dumb, 25)
	testsearch.CheckEqual(t, smart, smartDocs.ScoreDocs, dumbDocs.ScoreDocs)
}
