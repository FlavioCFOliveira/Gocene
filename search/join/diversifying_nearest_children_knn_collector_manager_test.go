package join

import "testing"

func TestDiversifyingChildKnnCollectorPicksBestPerParent(t *testing.T) {
	m := NewDiversifyingNearestChildrenKnnCollectorManager(3, nil)
	c := m.NewCollector()
	c.Collect(10, 11, 0.5)
	c.Collect(10, 12, 0.9)
	c.Collect(20, 21, 0.7)
	c.Collect(20, 22, 0.4)
	children, scores := c.Results()
	if len(children) != 2 {
		t.Fatalf("children = %v", children)
	}
	for i, child := range children {
		switch child {
		case 12:
			if scores[i] != 0.9 {
				t.Errorf("parent 10 should keep score 0.9, got %v", scores[i])
			}
		case 21:
			if scores[i] != 0.7 {
				t.Errorf("parent 20 should keep score 0.7, got %v", scores[i])
			}
		default:
			t.Errorf("unexpected child %d", child)
		}
	}
}
