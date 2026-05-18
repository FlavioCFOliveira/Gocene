package facets

import "testing"

func TestStringValueFacetCounts(t *testing.T) {
	state := NewStringDocValuesReaderState("brand", []string{"acme", "globex", "initech"})
	c := NewStringValueFacetCounts(state)
	c.IncrementOrd(0)
	c.IncrementOrd(0)
	c.IncrementOrd(1)
	c.IncrementOrd(2)
	c.IncrementOrd(2)
	c.IncrementOrd(2)
	if c.GetTotalCount() != 6 {
		t.Errorf("total = %d", c.GetTotalCount())
	}
	top := c.GetTopChildren(2)
	if len(top) != 2 {
		t.Fatalf("len = %d", len(top))
	}
	if top[0].Label != "initech" || top[0].Value != 3 {
		t.Errorf("top[0] = %v", top[0])
	}
	if top[1].Label != "acme" || top[1].Value != 2 {
		t.Errorf("top[1] = %v", top[1])
	}
}
