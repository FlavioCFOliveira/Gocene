package taxonomywritercache

import "testing"

func TestLruTaxonomyWriterCacheBasic(t *testing.T) {
	c := NewLruTaxonomyWriterCache(2)
	c.Put("a", 1)
	c.Put("b", 2)
	if c.Size() != 2 || !c.IsFull() {
		t.Errorf("size=%d full=%v", c.Size(), c.IsFull())
	}
	// "a" was inserted first; "b" sits at the front (MRU). Inserting "c"
	// pushes "a" out as the LRU.
	c.Put("c", 3)
	if c.Get("a") != -1 {
		t.Error("a should have been evicted")
	}
	if c.Get("b") != 2 || c.Get("c") != 3 {
		t.Error("b and c should survive")
	}
}

func TestLruTaxonomyWriterCacheClear(t *testing.T) {
	c := NewLruTaxonomyWriterCache(2)
	c.Put("a", 1)
	c.Clear()
	if c.Size() != 0 || c.Get("a") != -1 {
		t.Error("Clear")
	}
}
