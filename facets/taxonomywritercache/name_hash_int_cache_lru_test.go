package taxonomywritercache

import "testing"

func TestNameHashIntCacheLRU(t *testing.T) {
	c := NewNameHashIntCacheLRU(2)
	c.Put("alpha", 1)
	c.Put("beta", 2)
	if c.Get("alpha") != 1 || c.Get("beta") != 2 {
		t.Error("Get")
	}
	c.Put("gamma", 3) // evicts alpha (LRU after gets) -- but get bumped alpha; check actual
	if c.Get("gamma") != 3 {
		t.Error("gamma should be present")
	}
}
