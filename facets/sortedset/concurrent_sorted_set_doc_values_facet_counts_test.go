package sortedset

import (
	"sync"
	"testing"
)

func TestConcurrentSortedSetDocValuesFacetCountsRace(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("f", 4, map[string][2]int{
		"d": {0, 4},
	})
	c := NewConcurrentSortedSetDocValuesFacetCounts(state)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.IncrementOrd(0)
				c.IncrementOrd(1)
			}
		}()
	}
	wg.Wait()
	if c.CountForOrd(0) != 400 {
		t.Errorf("ord 0 = %d", c.CountForOrd(0))
	}
	if c.GetTotalCount() != 800 {
		t.Errorf("total = %d", c.GetTotalCount())
	}
	counts := c.CountsForDim("d")
	if len(counts) != 4 || counts[0] != 400 {
		t.Errorf("dim counts = %v", counts)
	}
}
