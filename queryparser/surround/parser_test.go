package surround

import (
	"testing"
)

func TestTokenManagerSimpleTerm(t *testing.T) {
	m := NewQueryParserTokenManager("hello")
	tok, err := m.NextToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != Term || tok.Image != "hello" {
		t.Errorf("got kind=%d image=%q", tok.Kind, tok.Image)
	}
	next, err := m.NextToken()
	if err != nil {
		t.Fatal(err)
	}
	if next.Kind != EOF {
		t.Errorf("expected EOF, got %d", next.Kind)
	}
}

func TestTokenManagerKeywords(t *testing.T) {
	m := NewQueryParserTokenManager("foo OR bar AND baz NOT qux")
	expectedKinds := []int{Term, OrOp, Term, AndOp, Term, NotOp, Term, EOF}
	for i, want := range expectedKinds {
		tok, err := m.NextToken()
		if err != nil {
			t.Fatal(err)
		}
		if tok.Kind != want {
			t.Errorf("token %d kind = %d, want %d (image=%q)", i, tok.Kind, want, tok.Image)
		}
	}
}

func TestTokenManagerDistance(t *testing.T) {
	m := NewQueryParserTokenManager("3W 5N 42")
	tok, _ := m.NextToken()
	if tok.Kind != W || tok.Image != "3W" {
		t.Errorf("3W: %v", tok)
	}
	tok, _ = m.NextToken()
	if tok.Kind != N || tok.Image != "5N" {
		t.Errorf("5N: %v", tok)
	}
	tok, _ = m.NextToken()
	if tok.Kind != NumberToken || tok.Image != "42" {
		t.Errorf("42: %v", tok)
	}
}

func TestTokenManagerStructural(t *testing.T) {
	m := NewQueryParserTokenManager("(foo,bar):baz^2")
	want := []int{OpenParen, Term, Comma, Term, CloseParen, Colon, Term, Caret, NumberToken, EOF}
	for i, w := range want {
		tok, _ := m.NextToken()
		if tok.Kind != w {
			t.Errorf("token %d kind=%d, want %d (image=%q)", i, tok.Kind, w, tok.Image)
		}
	}
}

func TestTokenManagerTruncterm(t *testing.T) {
	m := NewQueryParserTokenManager("hel*")
	tok, _ := m.NextToken()
	if tok.Kind != Truncterm {
		t.Errorf("kind = %d, want Truncterm", tok.Kind)
	}
}

func TestTokenManagerWildcard(t *testing.T) {
	m := NewQueryParserTokenManager("h?llo")
	tok, _ := m.NextToken()
	if tok.Kind != Suffixterm {
		t.Errorf("kind = %d, want Suffixterm", tok.Kind)
	}
}

func TestTokenManagerQuoted(t *testing.T) {
	m := NewQueryParserTokenManager(`"hello world"`)
	tok, _ := m.NextToken()
	if tok.Kind != QuotedToken {
		t.Errorf("kind = %d", tok.Kind)
	}
	if tok.Image != `"hello world"` {
		t.Errorf("image = %q", tok.Image)
	}
}

func TestParserSimpleTerm(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*SrndTermQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserOrExpression(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("foo OR bar OR baz")
	if err != nil {
		t.Fatal(err)
	}
	or, ok := q.(*OrQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(or.GetChildren()) != 3 {
		t.Errorf("children = %d", len(or.GetChildren()))
	}
}

func TestParserAndExpression(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("foo AND bar")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*AndQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserNotExpression(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("foo NOT bar")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*NotQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserDistanceExpression(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("foo 3W bar")
	if err != nil {
		t.Fatal(err)
	}
	dq, ok := q.(*DistanceQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if dq.GetOpDistance() != 3 {
		t.Errorf("distance = %d", dq.GetOpDistance())
	}
	if !dq.IsOrdered() {
		t.Error("3W should be ordered")
	}
}

func TestParserDistanceUnordered(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("foo 5N bar")
	if err != nil {
		t.Fatal(err)
	}
	dq := q.(*DistanceQuery)
	if dq.IsOrdered() {
		t.Error("5N should be unordered")
	}
}

func TestParserGroup(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("(foo OR bar) AND baz")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*AndQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserFieldQuery(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("title:hello")
	if err != nil {
		t.Fatal(err)
	}
	fq, ok := q.(*FieldsQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(fq.GetFieldNames()) != 1 || fq.GetFieldNames()[0] != "title" {
		t.Errorf("fields = %v", fq.GetFieldNames())
	}
}

func TestParserMultiFieldQuery(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("title,body:hello")
	if err != nil {
		t.Fatal(err)
	}
	fq, ok := q.(*FieldsQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(fq.GetFieldNames()) != 2 {
		t.Errorf("fields = %v", fq.GetFieldNames())
	}
}

func TestParserBoost(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("hello^2.5")
	if err != nil {
		t.Fatal(err)
	}
	if q.GetWeight() != 2.5 {
		t.Errorf("weight = %v", q.GetWeight())
	}
}

func TestParserQuoted(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse(`"hello world"`)
	if err != nil {
		t.Fatal(err)
	}
	tq, ok := q.(*SrndTermQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if !tq.IsQuoted() {
		t.Error("should be quoted")
	}
}

func TestParserTruncterm(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("hel*")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*SrndPrefixQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserWildcard(t *testing.T) {
	p := NewQueryParser("body")
	q, err := p.Parse("h?llo")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*SrndTruncQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestParserEmptyError(t *testing.T) {
	p := NewQueryParser("body")
	if _, err := p.Parse(""); err == nil {
		t.Error("expected error")
	}
}

func TestParserUnclosedParen(t *testing.T) {
	p := NewQueryParser("body")
	if _, err := p.Parse("(foo"); err == nil {
		t.Error("expected error")
	}
}
