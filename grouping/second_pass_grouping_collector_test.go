package grouping

import "testing"

func TestSecondPassGroupingCollectorRespectsTargets(t *testing.T) {
	c := NewSecondPassGroupingCollector[string]([]string{"red", "green"}, 3)
	c.Collect("red", 1)
	c.Collect("green", 2)
	c.Collect("blue", 3) // dropped
	c.Collect("red", 4)
	c.Collect("red", 5)
	c.Collect("red", 6) // dropped: budget exhausted
	if c.GroupCount() != 2 {
		t.Errorf("groups = %d", c.GroupCount())
	}
	red := c.GetDocs("red")
	if len(red) != 3 {
		t.Errorf("red = %v", red)
	}
	if len(c.GetDocs("blue")) != 0 {
		t.Error("blue should be empty")
	}
}
