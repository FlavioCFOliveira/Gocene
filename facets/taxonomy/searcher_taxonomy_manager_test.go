package taxonomy

import "testing"

func TestSearcherTaxonomyManager(t *testing.T) {
	initial := SearcherAndTaxonomy{Searcher: "s1", Taxonomy: "t1"}
	calls := 0
	m := NewSearcherTaxonomyManager(initial, func() (SearcherAndTaxonomy, error) {
		calls++
		return SearcherAndTaxonomy{Searcher: "s2", Taxonomy: "t2"}, nil
	})
	pair := m.Acquire()
	if pair.Searcher.(string) != "s1" {
		t.Errorf("first acquire = %v", pair)
	}
	if err := m.MaybeRefresh(); err != nil {
		t.Fatal(err)
	}
	pair = m.Acquire()
	if pair.Searcher.(string) != "s2" {
		t.Errorf("after refresh = %v", pair)
	}
	if calls != 1 {
		t.Errorf("refresh calls = %d", calls)
	}
}
