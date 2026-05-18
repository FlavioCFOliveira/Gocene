package grouping

import "testing"

func TestTopGroupsCollectorKeepsBestScores(t *testing.T) {
	c := NewTopGroupsCollector[string](2, nil) // descending score
	c.Collect("red", 1, 1.0)
	c.Collect("red", 2, 5.0)
	c.Collect("red", 3, 3.0) // evicts the 1.0 entry
	docs, scores := c.GetDocsAndScores("red")
	if len(docs) != 2 {
		t.Fatalf("docs = %v", docs)
	}
	have := map[float64]bool{}
	for _, s := range scores {
		have[s] = true
	}
	if !have[5.0] || !have[3.0] {
		t.Errorf("expected 5.0 + 3.0, got %v", scores)
	}
}
