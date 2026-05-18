package simple

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestSimpleParserEmptyReturnsMatchNoDocs(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("")
	if _, ok := q.(*search.MatchNoDocsQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserSingleTerm(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("hello")
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserAndOr(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("+foo -bar |baz")
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(bq.Clauses()) != 3 {
		t.Fatalf("clauses = %d", len(bq.Clauses()))
	}
	expected := []search.Occur{search.MUST, search.MUST_NOT, search.SHOULD}
	for i, c := range bq.Clauses() {
		if c.Occur != expected[i] {
			t.Errorf("clause %d occur = %v, want %v", i, c.Occur, expected[i])
		}
	}
}

func TestSimpleParserPhrase(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("\"hello world\"")
	if _, ok := q.(*search.PhraseQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserPhraseWithSlop(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("\"hello world\"~5")
	pq, ok := q.(*search.PhraseQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if pq.GetSlop() != 5 {
		t.Errorf("slop = %d", pq.GetSlop())
	}
}

func TestSimpleParserPrefix(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("foo*")
	if _, ok := q.(*search.PrefixQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserFuzzy(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("foo~1")
	if _, ok := q.(*search.FuzzyQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserParens(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "body")
	q := p.Parse("(foo bar) baz")
	if _, ok := q.(*search.BooleanQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSimpleParserMultipleFields(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "title", "body")
	q := p.Parse("hello")
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(bq.Clauses()) != 2 {
		t.Errorf("clauses = %d", len(bq.Clauses()))
	}
}

func TestSimpleParserFieldBoosts(t *testing.T) {
	p := NewSimpleQueryParser(analysis.NewStandardAnalyzer(), "title", "body")
	p.FieldWeights = map[string]float32{"title": 3.0}
	q := p.Parse("hello")
	bq := q.(*search.BooleanQuery)
	if _, ok := bq.Clauses()[0].Query.(*search.BoostQuery); !ok {
		t.Errorf("title clause should be boosted, got %T", bq.Clauses()[0].Query)
	}
}

func TestSimpleParserFlagsRestricted(t *testing.T) {
	p := NewSimpleQueryParserWithFlags(analysis.NewStandardAnalyzer(), []string{"body"}, OpAnd|OpNot)
	q := p.Parse("+foo |bar")
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	for _, c := range bq.Clauses() {
		if c.Occur == search.SHOULD {
			continue
		}
	}
	if len(bq.Clauses()) < 2 {
		t.Errorf("expected at least 2 clauses, got %d", len(bq.Clauses()))
	}
}
