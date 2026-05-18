package facets

import "testing"

func TestMultiFacetQuery(t *testing.T) {
	q := NewMultiFacetQuery(nil, "color", []string{"red"}, []string{"blue"})
	if q.GetDim() != "color" {
		t.Error("dim")
	}
	if len(q.GetPaths()) != 2 {
		t.Errorf("paths = %d", len(q.GetPaths()))
	}
	if len(q.Clauses()) != 2 {
		t.Errorf("clauses = %d", len(q.Clauses()))
	}
}
