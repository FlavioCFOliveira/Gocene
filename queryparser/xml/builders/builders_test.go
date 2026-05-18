package builders

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func parseDoc(t *testing.T, s string) *xml.Element {
	t.Helper()
	e, err := xml.ParseDocument(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestTermQueryBuilder(t *testing.T) {
	e := parseDoc(t, `<TermQuery fieldName="title">hello</TermQuery>`)
	q, err := TermQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestTermQueryBuilderMissingField(t *testing.T) {
	e := parseDoc(t, `<TermQuery>hello</TermQuery>`)
	if _, err := (TermQueryBuilder{}).GetQuery(e); err == nil {
		t.Error("expected error for missing fieldName")
	}
}

func TestTermsQueryBuilder(t *testing.T) {
	e := parseDoc(t, `<TermsQuery fieldName="title" minimumNumberShouldMatch="2">a b c</TermsQuery>`)
	q, err := TermsQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(bq.Clauses()) != 3 {
		t.Errorf("clauses = %d", len(bq.Clauses()))
	}
	if bq.MinimumNumberShouldMatch() != 2 {
		t.Errorf("min = %d", bq.MinimumNumberShouldMatch())
	}
}

func TestMatchAllDocsBuilder(t *testing.T) {
	e := parseDoc(t, `<MatchAllDocsQuery/>`)
	q, err := MatchAllDocsQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestBooleanQueryBuilder(t *testing.T) {
	factory := xml.NewQueryBuilderFactory()
	factory.AddBuilder("TermQuery", TermQueryBuilder{})
	factory.AddBuilder("BooleanQuery", NewBooleanQueryBuilder(factory))
	e := parseDoc(t, `<BooleanQuery>
		<Clause occurs="must"><TermQuery fieldName="t">a</TermQuery></Clause>
		<Clause occurs="should"><TermQuery fieldName="t">b</TermQuery></Clause>
		<Clause occurs="mustNot"><TermQuery fieldName="t">c</TermQuery></Clause>
	</BooleanQuery>`)
	q, err := factory.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	bq := q.(*search.BooleanQuery)
	if len(bq.Clauses()) != 3 {
		t.Fatalf("clauses = %d", len(bq.Clauses()))
	}
	occurs := []search.Occur{search.MUST, search.SHOULD, search.MUST_NOT}
	for i, c := range bq.Clauses() {
		if c.Occur != occurs[i] {
			t.Errorf("clause %d occur = %v", i, c.Occur)
		}
	}
}

func TestConstantScoreQueryBuilder(t *testing.T) {
	factory := xml.NewQueryBuilderFactory()
	factory.AddBuilder("TermQuery", TermQueryBuilder{})
	b := NewConstantScoreQueryBuilder(factory)
	e := parseDoc(t, `<ConstantScoreQuery boost="3.0"><TermQuery fieldName="t">x</TermQuery></ConstantScoreQuery>`)
	q, err := b.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.BoostQuery); !ok {
		t.Errorf("expected BoostQuery wrap, got %T", q)
	}
}

func TestDisjunctionMaxQueryBuilder(t *testing.T) {
	factory := xml.NewQueryBuilderFactory()
	factory.AddBuilder("TermQuery", TermQueryBuilder{})
	b := NewDisjunctionMaxQueryBuilder(factory)
	e := parseDoc(t, `<DisjunctionMaxQuery tieBreaker="0.5">
		<TermQuery fieldName="t">a</TermQuery>
		<TermQuery fieldName="t">b</TermQuery>
	</DisjunctionMaxQuery>`)
	q, err := b.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.DisjunctionMaxQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestRangeQueryBuilder(t *testing.T) {
	e := parseDoc(t, `<RangeQuery fieldName="t" lowerTerm="a" upperTerm="z" includeLower="true" includeUpper="false"/>`)
	q, err := RangeQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermRangeQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestPointRangeQueryBuilder(t *testing.T) {
	e := parseDoc(t, `<PointRangeQuery fieldName="t" lowerTerm="aaaa" upperTerm="zzzz"/>`)
	q, err := PointRangeQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.PointRangeQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestUserInputQueryBuilder(t *testing.T) {
	called := false
	b := NewUserInputQueryBuilder(func(field, queryString string) (search.Query, error) {
		called = true
		if field != "title" {
			t.Errorf("field = %q", field)
		}
		if queryString != "hello world" {
			t.Errorf("query = %q", queryString)
		}
		return search.NewMatchAllDocsQuery(), nil
	})
	e := parseDoc(t, `<UserQuery fieldName="title">hello world</UserQuery>`)
	q, err := b.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if q == nil || !called {
		t.Error("not invoked")
	}
}

func TestSpanTermBuilder(t *testing.T) {
	e := parseDoc(t, `<SpanTerm fieldName="t">hello</SpanTerm>`)
	q, err := SpanTermBuilder{}.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanTermQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSpanOrBuilder(t *testing.T) {
	factory := NewSpanQueryBuilderFactory()
	factory.AddBuilder("SpanTerm", SpanTermBuilder{})
	factory.AddBuilder("SpanOr", NewSpanOrBuilder(factory))
	e := parseDoc(t, `<SpanOr><SpanTerm fieldName="t">a</SpanTerm><SpanTerm fieldName="t">b</SpanTerm></SpanOr>`)
	q, err := factory.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanOrQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSpanOrTermsBuilder(t *testing.T) {
	e := parseDoc(t, `<SpanOrTerms fieldName="t">a b c</SpanOrTerms>`)
	q, err := SpanOrTermsBuilder{}.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Error("nil")
	}
}

func TestSpanNotBuilder(t *testing.T) {
	factory := NewSpanQueryBuilderFactory()
	factory.AddBuilder("SpanTerm", SpanTermBuilder{})
	factory.AddBuilder("SpanNot", NewSpanNotBuilder(factory))
	e := parseDoc(t, `<SpanNot>
		<Include><SpanTerm fieldName="t">a</SpanTerm></Include>
		<Exclude><SpanTerm fieldName="t">b</SpanTerm></Exclude>
	</SpanNot>`)
	q, err := factory.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanNotQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSpanNearBuilder(t *testing.T) {
	factory := NewSpanQueryBuilderFactory()
	factory.AddBuilder("SpanTerm", SpanTermBuilder{})
	factory.AddBuilder("SpanNear", NewSpanNearBuilder(factory))
	e := parseDoc(t, `<SpanNear slop="3" inOrder="true">
		<SpanTerm fieldName="t">a</SpanTerm>
		<SpanTerm fieldName="t">b</SpanTerm>
	</SpanNear>`)
	q, err := factory.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanNearQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSpanFirstBuilder(t *testing.T) {
	factory := NewSpanQueryBuilderFactory()
	factory.AddBuilder("SpanTerm", SpanTermBuilder{})
	factory.AddBuilder("SpanFirst", NewSpanFirstBuilder(factory))
	e := parseDoc(t, `<SpanFirst end="5"><SpanTerm fieldName="t">a</SpanTerm></SpanFirst>`)
	q, err := factory.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanFirstQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSpanPositionRangeBuilder(t *testing.T) {
	factory := NewSpanQueryBuilderFactory()
	factory.AddBuilder("SpanTerm", SpanTermBuilder{})
	factory.AddBuilder("SpanPositionRange", NewSpanPositionRangeBuilder(factory))
	e := parseDoc(t, `<SpanPositionRange start="0" end="5"><SpanTerm fieldName="t">a</SpanTerm></SpanPositionRange>`)
	q, err := factory.GetSpanQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanPositionRangeQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestRegisterCoreBuildersAttachesAll(t *testing.T) {
	parser := xml.NewCoreParser("body")
	RegisterCoreBuilders(parser, nil)
	for _, tag := range []string{
		"TermQuery", "TermsQuery", "MatchAllDocsQuery", "ConstantScoreQuery",
		"BooleanQuery", "RangeQuery", "PointRangeQuery", "DisjunctionMaxQuery",
		"UserQuery", "SpanTerm", "SpanOr", "SpanOrTerms", "SpanNot", "SpanNear",
		"SpanFirst", "SpanPositionRange", "BoostingTermQuery",
	} {
		if parser.GetBuilder(tag) == nil {
			t.Errorf("builder missing for <%s>", tag)
		}
	}
}

func TestCorePlusQueriesParser(t *testing.T) {
	parser := xml.NewCorePlusQueriesParser("body")
	RegisterCorePlusQueriesBuilders(parser, analysis.NewStandardAnalyzer(), nil)
	if parser.GetBuilder("LikeThisQuery") == nil {
		t.Error("LikeThisQuery missing")
	}
	if parser.GetBuilder("FuzzyLikeThisQuery") == nil {
		t.Error("FuzzyLikeThisQuery missing")
	}
}

func TestCorePlusExtensionsParser(t *testing.T) {
	parser := xml.NewCorePlusExtensionsParser("body")
	RegisterCorePlusExtensionsBuilders(parser, analysis.NewStandardAnalyzer(), nil)
	if parser.GetBuilder("BooleanQuery") == nil {
		t.Error("expected BooleanQuery to be wired through inheritance")
	}
}

func TestCoreParserParseDocument(t *testing.T) {
	parser := xml.NewCoreParser("body")
	RegisterCoreBuilders(parser, nil)
	q, err := parser.Parse(strings.NewReader(`<MatchAllDocsQuery/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestBoostAttribute(t *testing.T) {
	e := parseDoc(t, `<TermQuery fieldName="t" boost="2.5">a</TermQuery>`)
	q, err := TermQueryBuilder{}.GetQuery(e)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.BoostQuery); !ok {
		t.Errorf("expected BoostQuery, got %T", q)
	}
}
