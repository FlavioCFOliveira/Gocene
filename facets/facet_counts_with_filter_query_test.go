package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestFacetCountsWithFilterQuery(t *testing.T) {
	f := NewFacetCountsWithFilterQuery(nil)
	if f.HasFilter() {
		t.Error("HasFilter() = true, want false for nil filter")
	}
	if f.GetFastMatchQuery() != nil {
		t.Error("filter should start nil")
	}
	q := search.NewMatchAllDocsQuery()
	f.SetFastMatchQuery(q)
	if !f.HasFilter() {
		t.Error("HasFilter() = false after Set")
	}
	if f.GetFastMatchQuery() != q {
		t.Error("filter not retained")
	}
}
