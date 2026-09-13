package facets

import "testing"

func TestMultiFacetQuery(t *testing.T) {
	q := NewMultiFacetQuery(nil, "color", []string{"red"}, []string{"blue"})
	if got := q.GetTermsCount(); got != 2 {
		t.Errorf("terms = %d, want 2", got)
	}
	// MultiFacetQuery indexes into the dimension's drill-down field.
	if got := q.ToString(""); got == "" {
		t.Error("ToString returned an empty description")
	}
}
