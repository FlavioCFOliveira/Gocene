// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"strings"
	"testing"
)

// fakeTokenStream is a deterministic TokenStreamLike backed by a
// fixed sequence of tokens.
type fakeTokenStream struct {
	tokens []fakeToken
	idx    int
}

type fakeToken struct {
	bytes   []byte
	posIncr int
	posLen  int
}

func (f *fakeTokenStream) Reset() error           { f.idx = 0; return nil }
func (f *fakeTokenStream) Close() error           { return nil }
func (f *fakeTokenStream) TermBytes() []byte      { return f.tokens[f.idx-1].bytes }
func (f *fakeTokenStream) PositionIncrement() int { return f.tokens[f.idx-1].posIncr }
func (f *fakeTokenStream) PositionLength() int    { return f.tokens[f.idx-1].posLen }

func (f *fakeTokenStream) IncrementToken() (bool, error) {
	if f.idx >= len(f.tokens) {
		return false, nil
	}
	f.idx++
	return true, nil
}

// fakeAnalyzer splits text on whitespace.
type fakeAnalyzer struct{}

func (a *fakeAnalyzer) TokenStream(field, text string) (TokenStreamLike, error) {
	parts := strings.Fields(text)
	tokens := make([]fakeToken, len(parts))
	for i, p := range parts {
		tokens[i] = fakeToken{bytes: []byte(p), posIncr: 1, posLen: 1}
	}
	return &fakeTokenStream{tokens: tokens}, nil
}

// fakeQueryFactory records calls in a string log for assertions.
type fakeQueryFactory struct {
	log strings.Builder
}

type fakeBooleanQuery struct {
	clauses  []QueryLike
	minMatch int
}

func (b *fakeBooleanQuery) Clauses() []QueryLike { return b.clauses }

func (f *fakeQueryFactory) NewTermQuery(field string, term []byte) QueryLike {
	f.log.WriteString("term(" + field + "," + string(term) + ");")
	return "TermQuery:" + field + ":" + string(term)
}
func (f *fakeQueryFactory) NewBooleanQuery() QueryLike {
	f.log.WriteString("bq();")
	return &fakeBooleanQuery{}
}
func (f *fakeQueryFactory) AddBooleanClause(bq QueryLike, child QueryLike, occur Occur) {
	f.log.WriteString("addClause();")
	b := bq.(*fakeBooleanQuery)
	b.clauses = append(b.clauses, child)
}
func (f *fakeQueryFactory) SetMinShouldMatch(bq QueryLike, n int) {
	f.log.WriteString("setMinShouldMatch();")
	bq.(*fakeBooleanQuery).minMatch = n
}
func (f *fakeQueryFactory) NewPhraseQuery(field string, terms [][]byte, slop int) QueryLike {
	f.log.WriteString("phrase(" + field + ",")
	for _, t := range terms {
		f.log.WriteString(string(t) + "|")
	}
	f.log.WriteString(");")
	return "PhraseQuery:" + field + ":" + string(terms[0])
}

// TestQueryBuilder_CreateBooleanQuery_Single covers the single-token
// path which Java collapses into a TermQuery.
func TestQueryBuilder_CreateBooleanQuery_Single(t *testing.T) {
	f := &fakeQueryFactory{}
	qb, err := NewQueryBuilder(&fakeAnalyzer{}, f)
	if err != nil {
		t.Fatalf("NewQueryBuilder: %v", err)
	}
	q, err := qb.CreateBooleanQuery("body", "hello")
	if err != nil {
		t.Fatalf("CreateBooleanQuery: %v", err)
	}
	if q != "TermQuery:body:hello" {
		t.Fatalf("expected TermQuery, got %v", q)
	}
}

// TestQueryBuilder_CreateBooleanQuery_Multi covers the multi-position
// boolean path with SHOULD operator.
func TestQueryBuilder_CreateBooleanQuery_Multi(t *testing.T) {
	f := &fakeQueryFactory{}
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, f)
	q, err := qb.CreateBooleanQuery("body", "hello world")
	if err != nil {
		t.Fatalf("CreateBooleanQuery: %v", err)
	}
	bq, ok := q.(*fakeBooleanQuery)
	if !ok {
		t.Fatalf("expected fakeBooleanQuery, got %T", q)
	}
	if len(bq.clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(bq.clauses))
	}
}

// TestQueryBuilder_CreatePhraseQuery walks the quoted path which
// produces a PhraseQuery for multi-position input.
func TestQueryBuilder_CreatePhraseQuery(t *testing.T) {
	f := &fakeQueryFactory{}
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, f)
	q, err := qb.CreatePhraseQueryWithSlop("body", "the quick", 2)
	if err != nil {
		t.Fatalf("CreatePhraseQuery: %v", err)
	}
	s, ok := q.(string)
	if !ok || !strings.HasPrefix(s, "PhraseQuery:body:") {
		t.Fatalf("expected PhraseQuery, got %v", q)
	}
}

// TestQueryBuilder_CreateBooleanQuery_InvalidOperator covers the
// Java IllegalArgumentException analogue.
func TestQueryBuilder_CreateBooleanQuery_InvalidOperator(t *testing.T) {
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, &fakeQueryFactory{})
	_, err := qb.CreateBooleanQueryWithOperator("body", "x", OccurMustNot)
	if !errors.Is(err, ErrQueryBuilderInvalidOperator) {
		t.Fatalf("expected ErrQueryBuilderInvalidOperator, got %v", err)
	}
}

// TestQueryBuilder_CreateMinShouldMatchQuery_Fraction1 covers the
// shortcut where fraction == 1 produces a MUST boolean query.
func TestQueryBuilder_CreateMinShouldMatchQuery_Fraction1(t *testing.T) {
	f := &fakeQueryFactory{}
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, f)
	q, err := qb.CreateMinShouldMatchQuery("body", "a b c", 1.0)
	if err != nil {
		t.Fatalf("CreateMinShouldMatchQuery: %v", err)
	}
	if _, ok := q.(*fakeBooleanQuery); !ok {
		t.Fatalf("expected fakeBooleanQuery for fraction=1, got %T", q)
	}
}

// TestQueryBuilder_CreateMinShouldMatchQuery_OutOfRange exercises the
// validation branch.
func TestQueryBuilder_CreateMinShouldMatchQuery_OutOfRange(t *testing.T) {
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, &fakeQueryFactory{})
	for _, f := range []float32{-0.1, 1.1} {
		if _, err := qb.CreateMinShouldMatchQuery("body", "x", f); err == nil {
			t.Fatalf("expected error for fraction %g", f)
		}
	}
}

// TestQueryBuilder_Getters_Setters runs through the boolean-flag
// accessors.
func TestQueryBuilder_Getters_Setters(t *testing.T) {
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, &fakeQueryFactory{})
	if !qb.EnablePositionIncrements() {
		t.Fatalf("default EnablePositionIncrements should be true")
	}
	qb.SetEnablePositionIncrements(false)
	if qb.EnablePositionIncrements() {
		t.Fatalf("expected false after SetEnablePositionIncrements(false)")
	}
	if !qb.EnableGraphQueries() {
		t.Fatalf("default EnableGraphQueries should be true")
	}
	qb.SetEnableGraphQueries(false)
	if qb.EnableGraphQueries() {
		t.Fatalf("expected false after SetEnableGraphQueries(false)")
	}
	if qb.AutoGenerateMultiTermSynonymsPhraseQuery() {
		t.Fatalf("default AutoGenerateMultiTermSynonymsPhraseQuery should be false")
	}
	qb.SetAutoGenerateMultiTermSynonymsPhraseQuery(true)
	if !qb.AutoGenerateMultiTermSynonymsPhraseQuery() {
		t.Fatalf("expected true after setter")
	}
}

// TestTermAndBoost_DeepCopy verifies the record's deep-copy contract.
func TestTermAndBoost_DeepCopy(t *testing.T) {
	src := []byte("abc")
	tb := NewTermAndBoost(src, 1.5)
	src[0] = 'Z'
	if tb.Term[0] != 'a' {
		t.Fatalf("expected term to be deep-copied, got %q after mutation", tb.Term)
	}
	if tb.Boost != 1.5 {
		t.Fatalf("Boost=%g want 1.5", tb.Boost)
	}
}

// TestQueryBuilder_EmptyInput verifies the no-token edge case.
func TestQueryBuilder_EmptyInput(t *testing.T) {
	qb, _ := NewQueryBuilder(&fakeAnalyzer{}, &fakeQueryFactory{})
	q, err := qb.CreateBooleanQuery("body", "")
	if err != nil {
		t.Fatalf("CreateBooleanQuery: %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil for empty input, got %v", q)
	}
}
