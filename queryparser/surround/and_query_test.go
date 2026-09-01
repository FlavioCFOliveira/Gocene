package surround

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestAndQuery_MakeLuceneQueryField(t *testing.T) {
	fieldName := "text"
	qf := NewBasicQueryFactoryWithLimit(10)

	children := []SrndQuery{
		NewSrndTermQuery("apple", false),
		NewSrndTermQuery("banana", false),
	}

	andQuery := NewAndQuery(children, true, "AND")

	luceneQuery, err := andQuery.MakeLuceneQueryField(fieldName, qf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bq, ok := luceneQuery.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery, got %T", luceneQuery)
	}

	clauses := bq.Clauses()
	if len(clauses) != 2 {
		t.Errorf("expected 2 clauses, got %d", len(clauses))
	}

	for _, c := range clauses {
		if c.Occur != search.MUST {
			t.Errorf("expected MUST occur, got %v", c.Occur)
		}
	}
}

func TestAndQuery_String(t *testing.T) {
	children := []SrndQuery{
		NewSrndTermQuery("apple", false),
		NewSrndTermQuery("banana", false),
	}

	// Infix
	andInfix := NewAndQuery(children, true, "AND")
	if got := andInfix.String(); got != "(apple AND banana)" {
		t.Errorf("expected (apple AND banana), got %q", got)
	}

	// Prefix
	andPrefix := NewAndQuery(children, false, "AND")
	if got := andPrefix.String(); got != "AND(apple, banana)" {
		t.Errorf("expected AND(apple, banana), got %q", got)
	}
}
