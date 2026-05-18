package facets

import "testing"

func TestRandomSamplingFacetsCollectorKeepsAllUntilBudget(t *testing.T) {
	c := NewRandomSamplingFacetsCollector(5, 1)
	for i := 0; i < 5; i++ {
		c.Collect(i)
	}
	if len(c.Matches()) != 5 {
		t.Errorf("kept = %d", len(c.Matches()))
	}
}

func TestRandomSamplingFacetsCollectorBoundedSample(t *testing.T) {
	c := NewRandomSamplingFacetsCollector(3, 42)
	for i := 0; i < 100; i++ {
		c.Collect(i)
	}
	if len(c.Matches()) != 3 {
		t.Errorf("sample size = %d", len(c.Matches()))
	}
	if c.Seen() != 100 {
		t.Errorf("seen = %d", c.Seen())
	}
}
