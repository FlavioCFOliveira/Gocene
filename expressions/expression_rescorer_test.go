package expressions

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

type mockExpression struct {
	val float64
}

func (m *mockExpression) Evaluate(bindings Bindings, doc int) (float64, error) {
	return m.val, nil
}

func (m *mockExpression) GetSortField(bindings Bindings, reverse bool) *search.SortField {
	return search.NewSortField(search.SortFieldTypeDoc, false)
}

func (m *mockExpression) GetDoubleValuesSource(bindings Bindings) DoubleValuesSource {
	return &mockDoubleValuesSource{val: m.val}
}

type mockDoubleValuesSource struct {
	val float64
}

func (m *mockDoubleValuesSource) FloatVal(doc int) (float32, error) {
	return float32(m.val), nil
}

func (m *mockDoubleValuesSource) ToString(doc int) (string, error) {
	return "mock", nil
}

func (m *mockDoubleValuesSource) Explain(ctx any, doc int, super search.Explanation) (search.Explanation, error) {
	return search.NewExplanation(true, float32(m.val), "mock"), nil
}

func TestExpressionRescorer_Rescore(t *testing.T) {
	expr := &mockExpression{val: 10.0}
	bindings := make(Bindings)
	rescorer := NewExpressionRescorer(expr, bindings)

	topDocs := &search.TopDocs{
		TotalHits: 1,
		ScoreDocs: []*search.ScoreDoc{
			{Doc: 0, Score: 1.0},
		},
		MaxScore: 1.0,
	}

	// SortRescorer.Rescore just re-sorts. ExpressionRescorer inherits it.
	res, err := rescorer.Rescore(nil, topDocs)
	if err != nil {
		t.Fatalf("Rescore failed: %v", err)
	}
	if len(res.ScoreDocs) != 1 {
		t.Errorf("Expected 1 doc, got %d", len(res.ScoreDocs))
	}
}

func TestExpressionRescorer_Explain(t *testing.T) {
	expr := &mockExpression{val: 10.0}
	bindings := make(Bindings)
	rescorer := NewExpressionRescorer(expr, bindings)

	firstPass := search.NewExplanation(true, 1.0, "first pass")
	// We need a mock searcher and reader context
	// For now, we'll mock the behavior or provide minimal real objects.

	// This test might fail if searcher is nil, but we'll see.
	_, err := rescorer.Explain(nil, firstPass, 0)
	if err == nil {
		t.Error("Expected error due to nil searcher")
	}
}
