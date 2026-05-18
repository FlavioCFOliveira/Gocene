package surround

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestSrndQueryBaseWeight(t *testing.T) {
	var b SrndQueryBase
	if b.GetWeight() != 1.0 {
		t.Errorf("default weight = %v", b.GetWeight())
	}
	if b.IsWeighted() {
		t.Error("should not be weighted yet")
	}
	b.SetWeight(2.5)
	if b.GetWeight() != 2.5 {
		t.Errorf("after SetWeight = %v", b.GetWeight())
	}
	if !b.IsWeighted() {
		t.Error("should be weighted")
	}
	if !b.IsFieldsSubQueryAcceptable() {
		t.Error("default should be acceptable")
	}
}

func TestBasicQueryFactoryLimit(t *testing.T) {
	f := NewBasicQueryFactoryWithLimit(2)
	if _, err := f.MakeBasicTermQuery("x", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.MakeBasicTermQuery("x", "b"); err != nil {
		t.Fatal(err)
	}
	_, err := f.MakeBasicTermQuery("x", "c")
	var lim *TooManyBasicQueries
	if !errors.As(err, &lim) {
		t.Fatalf("expected TooManyBasicQueries, got %v", err)
	}
	if lim.MaxBasicQueries != 2 {
		t.Errorf("MaxBasicQueries = %d", lim.MaxBasicQueries)
	}
	if f.GetQueriesMade() != 3 {
		t.Errorf("count = %d", f.GetQueriesMade())
	}
}

func TestSrndTermQueryProducesTermQuery(t *testing.T) {
	factory := NewBasicQueryFactory()
	term := NewSrndTermQuery("hello", false)
	q, err := term.MakeLuceneQueryField("body", factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("expected TermQuery, got %T", q)
	}
	if term.GetTermText() != "hello" {
		t.Error("text")
	}
	if term.IsQuoted() {
		t.Error("quoted")
	}
}

func TestSrndPrefixQueryProducesPrefixQuery(t *testing.T) {
	factory := NewBasicQueryFactory()
	q, err := NewSrndPrefixQuery("ab", false, '*').MakeLuceneQueryField("body", factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.PrefixQuery); !ok {
		t.Errorf("expected PrefixQuery, got %T", q)
	}
}

func TestSrndTruncQueryProducesWildcardQuery(t *testing.T) {
	factory := NewBasicQueryFactory()
	q, err := NewSrndTruncQuery("h?l*o", '*', '?').MakeLuceneQueryField("body", factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.WildcardQuery); !ok {
		t.Errorf("expected WildcardQuery, got %T", q)
	}
}

func TestAndQueryBuildsMustClauses(t *testing.T) {
	factory := NewBasicQueryFactory()
	a := NewSrndTermQuery("foo", false)
	b := NewSrndTermQuery("bar", false)
	q, err := NewAndQuery([]SrndQuery{a, b}, true, "AND").MakeLuceneQueryField("body", factory)
	if err != nil {
		t.Fatal(err)
	}
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery, got %T", q)
	}
	if len(bq.Clauses()) != 2 {
		t.Errorf("clauses = %d", len(bq.Clauses()))
	}
	for _, c := range bq.Clauses() {
		if c.Occur != search.MUST {
			t.Errorf("occur = %v", c.Occur)
		}
	}
}

func TestOrQueryBuildsShouldClauses(t *testing.T) {
	factory := NewBasicQueryFactory()
	a := NewSrndTermQuery("foo", false)
	b := NewSrndTermQuery("bar", false)
	q, _ := NewOrQuery([]SrndQuery{a, b}, true, "OR").MakeLuceneQueryField("body", factory)
	bq := q.(*search.BooleanQuery)
	for _, c := range bq.Clauses() {
		if c.Occur != search.SHOULD {
			t.Errorf("occur = %v", c.Occur)
		}
	}
}

func TestNotQueryBuildsMustAndMustNot(t *testing.T) {
	factory := NewBasicQueryFactory()
	a := NewSrndTermQuery("foo", false)
	b := NewSrndTermQuery("bar", false)
	q, _ := NewNotQuery([]SrndQuery{a, b}, true, "NOT").MakeLuceneQueryField("body", factory)
	bq := q.(*search.BooleanQuery)
	if len(bq.Clauses()) != 2 {
		t.Fatalf("clauses = %d", len(bq.Clauses()))
	}
	if bq.Clauses()[0].Occur != search.MUST {
		t.Errorf("first = %v", bq.Clauses()[0].Occur)
	}
	if bq.Clauses()[1].Occur != search.MUST_NOT {
		t.Errorf("second = %v", bq.Clauses()[1].Occur)
	}
}

func TestDistanceQueryBuildsSpanNear(t *testing.T) {
	factory := NewBasicQueryFactory()
	a := NewSrndTermQuery("foo", false)
	b := NewSrndTermQuery("bar", false)
	dq := NewDistanceQuery([]SrndQuery{a, b}, true, 3, "3W", true)
	q, err := dq.MakeLuceneQueryField("body", factory)
	if err != nil {
		t.Fatal(err)
	}
	sn, ok := q.(*search.SpanNearQuery)
	if !ok {
		t.Fatalf("expected SpanNearQuery, got %T", q)
	}
	if !dq.IsOrdered() {
		t.Error("should be ordered")
	}
	if dq.GetOpDistance() != 3 {
		t.Errorf("distance = %d", dq.GetOpDistance())
	}
	_ = sn
}

func TestFieldsQueryDispatchesToMultipleFields(t *testing.T) {
	factory := NewBasicQueryFactory()
	sub := NewSrndTermQuery("hello", false)
	fq := NewFieldsQuery(sub, []string{"title", "body"}, '/')
	if fq.IsFieldsSubQueryAcceptable() {
		t.Error("FieldsQuery should reject sub-query nesting")
	}
	q, err := fq.MakeLuceneQueryField("ignored", factory)
	if err != nil {
		t.Fatal(err)
	}
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("got %T", q)
	}
	if len(bq.Clauses()) != 2 {
		t.Errorf("clauses = %d", len(bq.Clauses()))
	}
}

func TestFieldsQuerySingleFieldDelegates(t *testing.T) {
	factory := NewBasicQueryFactory()
	sub := NewSrndTermQuery("hello", false)
	fq := NewFieldsQuery(sub, []string{"title"}, '/')
	q, err := fq.MakeLuceneQueryField("ignored", factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("single field should delegate to TermQuery, got %T", q)
	}
}

func TestSpanNearClauseFactoryDeduplicates(t *testing.T) {
	factory := NewBasicQueryFactory()
	f := NewSpanNearClauseFactory("body", factory)
	if err := f.AddTermWeighted("foo", 1.0); err != nil {
		t.Fatal(err)
	}
	if err := f.AddTermWeighted("foo", 1.0); err != nil {
		t.Fatal(err)
	}
	if f.Size() != 1 {
		t.Errorf("size = %d, want 1", f.Size())
	}
	f.Clear()
	if f.Size() != 0 {
		t.Errorf("after clear = %d", f.Size())
	}
}

func TestComposedQueryGetters(t *testing.T) {
	a := NewSrndTermQuery("foo", false)
	c := NewComposedQuery([]SrndQuery{a}, true, "AND")
	if c.GetOperatorName() != "AND" {
		t.Error("op")
	}
	if !c.IsInfix() {
		t.Error("infix")
	}
	if len(c.GetChildren()) != 1 {
		t.Error("children")
	}
	c.SetChildren([]SrndQuery{a, a})
	if len(c.GetChildren()) != 2 {
		t.Error("after SetChildren")
	}
}
